package hn

import (
	"context"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const searchHits = 50

// enoughExact: sorted by date, fewer exact-word matches than this and the
// typo-tolerant ones are merged in — a misspelled query ("kubernets") has a
// few exact matches, all misspelled the same way, and misses what was meant.
const enoughExact = 10

// SearchKind says how a search found its results.
type SearchKind int

const (
	SearchMatched SearchKind = iota // Algolia: every word (typo-tolerant when needed)
	SearchAnyWord                   // Algolia: no title had every word; closest by any word
	SearchLoaded                    // local fuzzy match over stories loaded this session
)

// SearchOptions narrow a search, like the filters on hn.algolia.com.
type SearchOptions struct {
	Tag    string // Algolia tag: story (default), ask_hn, show_hn, launch_hn, job, poll
	ByDate bool   // newest first; otherwise by popularity (relevance, then points)
	Since  int64  // unix seconds; only items created after it (0: all time)
}

func (o SearchOptions) tag() string {
	if o.Tag == "" {
		return "story"
	}
	return o.Tag
}

type algoliaHit struct {
	ObjectID    string   `json:"objectID"`
	Title       *string  `json:"title"`
	URL         *string  `json:"url"`
	Author      string   `json:"author"`
	Points      *int     `json:"points"`
	NumComments *int     `json:"num_comments"`
	CreatedAtI  int64    `json:"created_at_i"`
	Tags        []string `json:"_tags"`
}

// search attempt strictness
type strictness int

const (
	exactWords strictness = iota // every word, no typos
	typoWords                    // every word, typo-tolerant
	anyWords                     // any word, typo-tolerant
)

// Search finds stories for a query, relaxing step by step until something
// matches: every word, then (for multi-word queries) any word, then a local
// fuzzy match over the stories already loaded — which is also the answer
// when Algolia can't be reached. Sorted by date, the first step is exact
// words only: typo matches aren't ranked below exact ones by date, and would
// crowd them out.
//
// An empty query with non-default options browses: "Show HN, past week, by
// popularity" needs no words.
func (c *Client) Search(ctx context.Context, query string, opt SearchOptions) ([]Item, SearchKind, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		items, err := c.algoliaSearch(ctx, "", opt, typoWords)
		return items, SearchMatched, err
	}
	steps := []strictness{typoWords}
	if opt.ByDate {
		steps = []strictness{exactWords, typoWords}
	}
	if len(strings.Fields(query)) > 1 {
		steps = append(steps, anyWords)
	}
	var err error
	var exact []Item
	for _, step := range steps {
		var items []Item
		if items, err = c.algoliaSearch(ctx, query, opt, step); err != nil {
			break
		}
		if step == exactWords {
			if len(items) >= enoughExact {
				return items, SearchMatched, nil
			}
			exact = items
			continue
		}
		if step == typoWords && len(exact) > 0 {
			return mergeByDate(exact, items), SearchMatched, nil
		}
		if len(items) > 0 {
			kind := SearchMatched
			if step == anyWords {
				kind = SearchAnyWord
				if opt.ByDate {
					sort.SliceStable(items, func(i, j int) bool { return items[i].Time > items[j].Time })
				}
			}
			return items, kind, nil
		}
	}
	if ctx.Err() != nil {
		return nil, SearchMatched, ctx.Err()
	}
	if local := FuzzyRank(query, filterLocal(c.cachedStories(), opt), searchHits); len(local) > 0 {
		if opt.ByDate {
			sort.SliceStable(local, func(i, j int) bool { return local[i].Time > local[j].Time })
		}
		return local, SearchLoaded, nil
	}
	return nil, SearchMatched, err
}

func (c *Client) algoliaSearch(ctx context.Context, query string, opt SearchOptions, step strictness) ([]Item, error) {
	q := url.Values{
		"query":       {query},
		"tags":        {opt.tag()},
		"hitsPerPage": {strconv.Itoa(searchHits)},
		// titles only: matching URLs and story text lets typo tolerance
		// stitch together weak "every word" matches, which then block the
		// much better any-word fallback
		"restrictSearchableAttributes": {"title"},
	}
	switch step {
	case exactWords:
		q.Set("typoTolerance", "false")
	case anyWords:
		q.Set("optionalWords", query)
	}
	if opt.Since > 0 {
		q.Set("numericFilters", "created_at_i>"+strconv.FormatInt(opt.Since, 10))
	}
	// any-word matches come from the relevance endpoint even when sorting by
	// date: by date, a story sharing one common word would rank with one
	// sharing them all. Search sorts the relevant ones by date afterwards.
	endpoint := "/search?"
	if opt.ByDate && step != anyWords {
		endpoint = "/search_by_date?"
	}
	var res struct {
		Hits []algoliaHit `json:"hits"`
	}
	if err := c.getJSON(ctx, c.AlgoliaURL+endpoint+q.Encode(), &res); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(res.Hits))
	for _, h := range res.Hits {
		id, err := strconv.Atoi(h.ObjectID)
		if err != nil || h.Title == nil || *h.Title == "" {
			continue
		}
		it := Item{ID: id, Type: "story", By: h.Author, Time: h.CreatedAtI, Title: *h.Title}
		for _, t := range h.Tags {
			if t == "job" || t == "poll" {
				it.Type = t
			}
		}
		if h.URL != nil {
			it.URL = *h.URL
		}
		if h.Points != nil {
			it.Score = *h.Points
		}
		if h.NumComments != nil {
			it.Descendants = *h.NumComments
		}
		it.sanitize()
		// not cached: search hits carry no kids, and the cache must hold whole items
		items = append(items, it)
	}
	return items, nil
}

// mergeByDate combines two result lists without duplicates, newest first.
func mergeByDate(a, b []Item) []Item {
	seen := make(map[int]bool, len(a)+len(b))
	var out []Item
	for _, it := range append(append([]Item{}, a...), b...) {
		if !seen[it.ID] {
			seen[it.ID] = true
			out = append(out, it)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time > out[j].Time })
	return out[:min(len(out), searchHits)]
}

// filterLocal applies search options to loaded items, for the offline fallback.
func filterLocal(items []Item, opt SearchOptions) []Item {
	var out []Item
	for _, it := range items {
		if opt.Since > 0 && it.Time <= opt.Since {
			continue
		}
		title := strings.ToLower(it.Title)
		ok := true
		switch opt.tag() {
		case "ask_hn":
			ok = strings.HasPrefix(title, "ask hn")
		case "show_hn":
			ok = strings.HasPrefix(title, "show hn")
		case "launch_hn":
			ok = strings.HasPrefix(title, "launch hn")
		case "job", "poll":
			ok = it.Type == opt.tag()
		default:
			ok = it.Type == "story"
		}
		if ok {
			out = append(out, it)
		}
	}
	return out
}

// cachedStories lists the stories in the item cache, newest first.
func (c *Client) cachedStories() []Item {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []Item
	for _, ci := range c.items {
		it := ci.item
		if it.Title != "" && !it.IsComment() && !it.Dead && !it.Deleted {
			out = append(out, it)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time > out[j].Time })
	return out
}

// FuzzyRank returns the items whose titles fuzzy-match every word of the
// query, best first. A word matches when its letters appear in the title in
// order ("borow" matches "borrow", "k8" matches "kubernetes 1.8"); matches at
// word starts and runs of consecutive letters score higher.
func FuzzyRank(query string, items []Item, limit int) []Item {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return nil
	}
	type scored struct {
		item  Item
		score int
	}
	var hits []scored
	for _, it := range items {
		title := []rune(strings.ToLower(it.Title))
		total := 0
		ok := true
		for _, w := range words {
			s, match := fuzzyScore([]rune(w), title)
			if !match {
				ok = false
				break
			}
			total += s
		}
		if ok {
			hits = append(hits, scored{it, total})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	out := make([]Item, 0, min(limit, len(hits)))
	for _, h := range hits[:min(limit, len(hits))] {
		out = append(out, h.item)
	}
	return out
}

// fuzzyScore matches pattern as a subsequence of text (both lowercased),
// greedily from the left.
func fuzzyScore(pattern, text []rune) (int, bool) {
	score, pi, last := 0, 0, -2
	for ti := 0; ti < len(text) && pi < len(pattern); ti++ {
		if text[ti] != pattern[pi] {
			continue
		}
		score++
		if ti == 0 || !unicode.IsLetter(text[ti-1]) && !unicode.IsDigit(text[ti-1]) {
			score += 5 // start of a word
		}
		if ti == last+1 {
			score += 3 // continues the previous match
		} else if last >= 0 {
			score -= min(ti-last-1, 3) // small penalty for gaps
		}
		last = ti
		pi++
	}
	return score, pi == len(pattern)
}
