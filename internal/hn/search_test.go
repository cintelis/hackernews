package hn

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	items, kind, err := c.Search(context.Background(), "  rust borow ")
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
	items, kind, err := c.Search(context.Background(), "rust borrow zebra")
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

	items, kind, err := c.Search(context.Background(), "kbrnts")
	if err != nil || kind != SearchLoaded || len(items) != 1 || items[0].ID != 5 {
		t.Fatalf("items %v kind %v err %v", items, kind, err)
	}
	if _, _, err := c.Search(context.Background(), "zzz"); err == nil {
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
