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

// SearchKind says how a search found its results.
type SearchKind int

const (
	SearchMatched SearchKind = iota // Algolia: every word, typo-tolerant
	SearchAnyWord                   // Algolia: no story had every word; closest by any word
	SearchLoaded                    // local fuzzy match over stories loaded this session
)

type algoliaHit struct {
	ObjectID    string  `json:"objectID"`
	Title       *string `json:"title"`
	URL         *string `json:"url"`
	Author      string  `json:"author"`
	Points      *int    `json:"points"`
	NumComments *int    `json:"num_comments"`
	CreatedAtI  int64   `json:"created_at_i"`
}

// Search finds stories for a query, relaxing step by step: Algolia with
// every word (it tolerates typos and matches word prefixes), then Algolia
// with any word, then a local fuzzy match over the stories already loaded —
// which is also the answer when Algolia can't be reached.
func (c *Client) Search(ctx context.Context, query string) ([]Item, SearchKind, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, SearchMatched, nil
	}
	items, err := c.algoliaSearch(ctx, query, false)
	if err == nil && len(items) > 0 {
		return items, SearchMatched, nil
	}
	if err == nil && len(strings.Fields(query)) > 1 {
		if items, err = c.algoliaSearch(ctx, query, true); err == nil && len(items) > 0 {
			return items, SearchAnyWord, nil
		}
	}
	if ctx.Err() != nil {
		return nil, SearchMatched, ctx.Err()
	}
	if local := FuzzyRank(query, c.cachedStories(), searchHits); len(local) > 0 {
		return local, SearchLoaded, nil
	}
	return nil, SearchMatched, err
}

func (c *Client) algoliaSearch(ctx context.Context, query string, anyWord bool) ([]Item, error) {
	q := url.Values{
		"query":       {query},
		"tags":        {"story"},
		"hitsPerPage": {strconv.Itoa(searchHits)},
		// titles only: matching URLs and story text lets typo tolerance
		// stitch together weak "every word" matches, which then block the
		// much better any-word fallback
		"restrictSearchableAttributes": {"title"},
	}
	if anyWord {
		q.Set("optionalWords", query)
	}
	var res struct {
		Hits []algoliaHit `json:"hits"`
	}
	if err := c.getJSON(ctx, c.AlgoliaURL+"/search?"+q.Encode(), &res); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(res.Hits))
	for _, h := range res.Hits {
		id, err := strconv.Atoi(h.ObjectID)
		if err != nil || h.Title == nil || *h.Title == "" {
			continue
		}
		it := Item{ID: id, Type: "story", By: h.Author, Time: h.CreatedAtI, Title: *h.Title}
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
