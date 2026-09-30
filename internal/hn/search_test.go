package hn

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func hit(id, title string) map[string]any {
	return map[string]any{"objectID": id, "title": title, "author": "a", "points": 10, "num_comments": 3, "created_at_i": 1, "url": "https://x.com/" + id}
}

// searchServer answers /search: strict queries get strict, anyWord ones
// (optionalWords set) get relaxed; a nil list is a server error.
func searchServer(t *testing.T, strict, relaxed []any) (*Client, *[]string) {
	t.Helper()
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		seen = append(seen, q.Get("query")+"|"+q.Get("optionalWords")+"|"+q.Get("tags")+"|"+q.Get("restrictSearchableAttributes"))
		hits := strict
		if q.Get("optionalWords") != "" {
			hits = relaxed
		}
		if hits == nil {
			http.Error(w, "down", http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"hits": hits})
	}))
	t.Cleanup(srv.Close)
	c := NewClient()
	c.AlgoliaURL = srv.URL
	return c, &seen
}

func TestSearchMatched(t *testing.T) {
	c, seen := searchServer(t, []any{hit("1", "Rust borrow checker"), hit("x", "bad id"), map[string]any{"objectID": "3"}}, nil)
	items, kind, err := c.Search(context.Background(), "  rust borow ", SearchOptions{})
	if err != nil || kind != SearchMatched || len(items) != 1 {
		t.Fatalf("items %+v kind %v err %v", items, kind, err)
	}
	it := items[0]
	if it.ID != 1 || it.Score != 10 || it.Descendants != 3 || it.URL != "https://x.com/1" {
		t.Errorf("hit converted wrong: %+v", it)
	}
	if (*seen)[0] != "rust borow||story|title" {
		t.Errorf("request = %q", (*seen)[0])
	}
}

func TestSearchFallsBackToAnyWord(t *testing.T) {
	c, seen := searchServer(t, []any{}, []any{hit("2", "Rust borrow checker")})
	items, kind, err := c.Search(context.Background(), "rust borrow zebra", SearchOptions{})
	if err != nil || kind != SearchAnyWord || len(items) != 1 {
		t.Fatalf("items %v kind %v err %v", items, kind, err)
	}
	if len(*seen) != 2 || (*seen)[1] != "rust borrow zebra|rust borrow zebra|story|title" {
		t.Errorf("requests = %q", *seen)
	}
}

func TestSearchFallsBackToLoadedStories(t *testing.T) {
	c, _ := searchServer(t, nil, nil) // search is down
	c.items[5] = cachedItem{item: Item{ID: 5, Type: "story", Title: "Kubernetes 1.8 released", Time: 2}}
	c.items[6] = cachedItem{item: Item{ID: 6, Type: "story", Title: "Unrelated"}}
	c.items[7] = cachedItem{item: Item{ID: 7, Type: "comment", Title: "kubernetes"}}

	items, kind, err := c.Search(context.Background(), "kbrnts", SearchOptions{})
	if err != nil || kind != SearchLoaded || len(items) != 1 || items[0].ID != 5 {
		t.Fatalf("items %v kind %v err %v", items, kind, err)
	}
	if _, _, err := c.Search(context.Background(), "zzz", SearchOptions{}); err == nil {
		t.Fatal("with search down and no local match, the error should surface")
	}
}

func TestFuzzyRank(t *testing.T) {
	items := []Item{
		{ID: 1, Title: "A backup tool"},
		{ID: 2, Title: "Rust borrow checker explained"},
		{ID: 3, Title: "Borrowing money for a car"},
	}
	got := FuzzyRank("borow rust", items, 10)
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("every word must match: %+v", got)
	}
	got = FuzzyRank("borrow", items, 10)
	if len(got) != 2 || got[0].ID != 3 && got[0].ID != 2 {
		t.Fatalf("got %+v", got)
	}
	// a word-start, contiguous match beats letters scattered through a word
	a, _ := fuzzyScore([]rune("tool"), []rune("a backup tool"))
	b, _ := fuzzyScore([]rune("tool"), []rune("the old optimal"))
	if a <= b {
		t.Errorf("contiguous %d should beat scattered %d", a, b)
	}
	if got := FuzzyRank("", items, 10); got != nil {
		t.Errorf("empty query: %v", got)
	}
}

// optionsServer records each request's path and parameters, answering from
// a function of them.
func optionsServer(t *testing.T, answer func(path string, q url.Values) []any) (*Client, *[]string) {
	t.Helper()
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		seen = append(seen, fmt.Sprintf("%s q=%q tags=%s typo=%s any=%q since=%s",
			r.URL.Path, q.Get("query"), q.Get("tags"), q.Get("typoTolerance"), q.Get("optionalWords"), q.Get("numericFilters")))
		json.NewEncoder(w).Encode(map[string]any{"hits": answer(r.URL.Path, q)})
	}))
	t.Cleanup(srv.Close)
	c := NewClient()
	c.AlgoliaURL = srv.URL
	return c, &seen
}

func TestSearchByDateTriesExactWordsFirst(t *testing.T) {
	c, seen := optionsServer(t, func(path string, q url.Values) []any {
		if q.Get("typoTolerance") == "false" {
			return []any{} // nothing spelled exactly like that
		}
		return []any{hit("9", "Kubernetes on bare metal")}
	})
	opt := SearchOptions{Tag: "show_hn", ByDate: true, Since: 1700000000}
	items, kind, err := c.Search(context.Background(), "kubernets", opt)
	if err != nil || kind != SearchMatched || len(items) != 1 {
		t.Fatalf("items %v kind %v err %v", items, kind, err)
	}
	want := []string{
		`/search_by_date q="kubernets" tags=show_hn typo=false any="" since=created_at_i>1700000000`,
		`/search_by_date q="kubernets" tags=show_hn typo= any="" since=created_at_i>1700000000`,
	}
	if len(*seen) != 2 || (*seen)[0] != want[0] || (*seen)[1] != want[1] {
		t.Fatalf("requests:\n%s", strings.Join(*seen, "\n"))
	}
}

func TestSearchByPopularityKeepsTypos(t *testing.T) {
	c, seen := optionsServer(t, func(string, url.Values) []any { return []any{hit("1", "x")} })
	c.Search(context.Background(), "kubernets", SearchOptions{})
	if len(*seen) != 1 || (*seen)[0] != `/search q="kubernets" tags=story typo= any="" since=` {
		t.Fatalf("requests: %q", *seen)
	}
}

func TestSearchEmptyQueryBrowses(t *testing.T) {
	c, seen := optionsServer(t, func(string, url.Values) []any {
		return []any{hit("1", "Show HN: a"), hit("2", "Show HN: b")}
	})
	items, kind, err := c.Search(context.Background(), "  ", SearchOptions{Tag: "show_hn", Since: 5})
	if err != nil || kind != SearchMatched || len(items) != 2 || len(*seen) != 1 {
		t.Fatalf("items %d kind %v err %v requests %q", len(items), kind, err, *seen)
	}
}

func TestSearchJobTypeKept(t *testing.T) {
	c, _ := optionsServer(t, func(string, url.Values) []any {
		h := hit("3", "Acme is hiring")
		h["_tags"] = []string{"job", "author_acme"}
		return []any{h}
	})
	items, _, _ := c.Search(context.Background(), "", SearchOptions{Tag: "job"})
	if len(items) != 1 || items[0].Type != "job" {
		t.Fatalf("items %+v", items)
	}
}

func TestFilterLocal(t *testing.T) {
	items := []Item{
		{ID: 1, Type: "story", Title: "Show HN: a tool", Time: 100},
		{ID: 2, Type: "story", Title: "Ask HN: a question", Time: 200},
		{ID: 3, Type: "job", Title: "Acme is hiring", Time: 300},
		{ID: 4, Type: "story", Title: "Plain story", Time: 50},
	}
	ids := func(opt SearchOptions) []int {
		var out []int
		for _, it := range filterLocal(items, opt) {
			out = append(out, it.ID)
		}
		return out
	}
	if got := ids(SearchOptions{Tag: "show_hn"}); len(got) != 1 || got[0] != 1 {
		t.Errorf("show_hn: %v", got)
	}
	if got := ids(SearchOptions{Tag: "job"}); len(got) != 1 || got[0] != 3 {
		t.Errorf("job: %v", got)
	}
	if got := ids(SearchOptions{Since: 90}); len(got) != 2 {
		t.Errorf("stories since 90: %v", got)
	}
}

func TestSearchByDateAnyWordUsesRelevanceThenDate(t *testing.T) {
	c, seen := optionsServer(t, func(path string, q url.Values) []any {
		if q.Get("optionalWords") == "" {
			return []any{} // no title has every word
		}
		older, newer := hit("1", "Rust borrow checker"), hit("2", "Rust async")
		older["created_at_i"], newer["created_at_i"] = 100, 200
		return []any{older, newer} // relevance order
	})
	items, kind, _ := c.Search(context.Background(), "rust borrow zebra", SearchOptions{ByDate: true})
	if kind != SearchAnyWord || len(items) != 2 || items[0].ID != 2 {
		t.Fatalf("kind %v items %+v", kind, items)
	}
	if last := (*seen)[len(*seen)-1]; !strings.HasPrefix(last, "/search q=") {
		t.Fatalf("any-word step should use the relevance endpoint: %q", last)
	}
}

func TestSearchByDateMergesTyposWhenExactIsThin(t *testing.T) {
	c, _ := optionsServer(t, func(path string, q url.Values) []any {
		if q.Get("typoTolerance") == "false" {
			h := hit("1", "ECS vs. Kubernets") // the one literal misspelling
			h["created_at_i"] = 100
			return []any{h}
		}
		a, b := hit("1", "ECS vs. Kubernets"), hit("2", "Kubernetes 2.0")
		a["created_at_i"], b["created_at_i"] = 100, 900
		return []any{a, b}
	})
	items, kind, err := c.Search(context.Background(), "kubernets", SearchOptions{ByDate: true})
	if err != nil || kind != SearchMatched || len(items) != 2 || items[0].ID != 2 || items[1].ID != 1 {
		t.Fatalf("want the typo match merged in, newest first, no duplicates: %+v (kind %v, err %v)", items, kind, err)
	}
}

func TestSearchByDateEnoughExactStops(t *testing.T) {
	c, seen := optionsServer(t, func(string, url.Values) []any {
		var hits []any
		for i := range enoughExact {
			hits = append(hits, hit(fmt.Sprint(i+1), "Rust thing"))
		}
		return hits
	})
	c.Search(context.Background(), "rust", SearchOptions{ByDate: true})
	if len(*seen) != 1 {
		t.Fatalf("enough exact matches should need one request: %q", *seen)
	}
}
