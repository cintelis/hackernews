package hn

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeHN serves both APIs from maps; algolia nil makes Algolia fail with 500.
type fakeHN struct {
	items   map[int]any
	algolia map[int]any
	hits    atomic.Int64
}

func (f *fakeHN) server(t *testing.T) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		var id int
		switch {
		case r.URL.Path == "/fb/topstories.json":
			json.NewEncoder(w).Encode([]int{1, 2, 3})
		case strings.HasPrefix(r.URL.Path, "/fb/item/"):
			if _, err := fmtSscanf(r.URL.Path, "/fb/item/%d.json", &id); err != nil {
				http.NotFound(w, r)
				return
			}
			json.NewEncoder(w).Encode(f.items[id]) // missing → null, like the real API
		case strings.HasPrefix(r.URL.Path, "/algolia/items/"):
			if f.algolia == nil {
				http.Error(w, "down", http.StatusInternalServerError)
				return
			}
			fmtSscanf(r.URL.Path, "/algolia/items/%d", &id)
			v, ok := f.algolia[id]
			if !ok {
				http.NotFound(w, r)
				return
			}
			json.NewEncoder(w).Encode(v)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	c := NewClient()
	c.FirebaseURL = srv.URL + "/fb"
	c.AlgoliaURL = srv.URL + "/algolia"
	return c
}

func item(id int, typ string, extra map[string]any) map[string]any {
	m := map[string]any{"id": id, "type": typ, "by": "u", "time": 1}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func TestFeedAndItems(t *testing.T) {
	f := &fakeHN{items: map[int]any{
		1: item(1, "story", map[string]any{"title": "one"}),
		2: item(2, "story", map[string]any{"deleted": true}),
		3: item(3, "story", map[string]any{"title": "three\x1b[31m"}),
	}}
	c := f.server(t)
	ctx := context.Background()

	ids, err := c.Feed(ctx, "top")
	if err != nil || len(ids) != 3 {
		t.Fatalf("Feed = %v, %v", ids, err)
	}
	items := c.Items(ctx, []int{3, 2, 1, 99})
	if len(items) != 2 || items[0].ID != 3 || items[1].ID != 1 {
		t.Fatalf("Items kept order / dropped deleted+missing wrong: %+v", items)
	}
	if items[0].Title != "three[31m" {
		t.Errorf("title not sanitized: %q", items[0].Title)
	}
}

func TestItemCacheAndInvalidate(t *testing.T) {
	f := &fakeHN{items: map[int]any{1: item(1, "story", nil)}}
	c := f.server(t)
	ctx := context.Background()
	c.Item(ctx, 1)
	c.Item(ctx, 1)
	if n := f.hits.Load(); n != 1 {
		t.Fatalf("cached item refetched: %d requests", n)
	}
	c.Invalidate(1)
	c.Item(ctx, 1)
	if n := f.hits.Load(); n != 2 {
		t.Fatalf("invalidated item not refetched: %d requests", n)
	}
	if _, err := c.Item(ctx, 42); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing item: err = %v, want ErrNotFound", err)
	}
}

func algoliaNode(id int, author any, text string, children ...any) map[string]any {
	return map[string]any{"id": id, "type": "comment", "author": author, "text": text, "created_at_i": 1, "children": children}
}

func TestThreadFromAlgolia(t *testing.T) {
	f := &fakeHN{algolia: map[int]any{
		10: map[string]any{"id": 10, "type": "story", "children": []any{
			algoliaNode(11, "a", "first"),
			algoliaNode(12, "b", "<p>second", algoliaNode(13, "c", "reply")),
			algoliaNode(14, nil, "", algoliaNode(15, "d", "orphan")), // deleted with a live reply
			algoliaNode(16, nil, ""),                                 // deleted, no replies
		}},
	}}
	c := f.server(t)
	tree, err := c.Thread(context.Background(), Item{ID: 10, Kids: []int{12, 11, 14}})
	if err != nil {
		t.Fatal(err)
	}
	var got []int
	for _, cm := range tree {
		got = append(got, cm.ID)
	}
	if want := []int{12, 11, 14}; !equalInts(got, want) {
		t.Fatalf("top-level order = %v, want HN ranking %v", got, want)
	}
	if tree[0].Children[0].Text != "reply" {
		t.Errorf("nested reply lost")
	}
	if !tree[2].Deleted || tree[2].Children[0].ID != 15 {
		t.Errorf("deleted comment with replies should be a placeholder")
	}
}

func TestThreadFallsBackToFirebase(t *testing.T) {
	f := &fakeHN{items: map[int]any{
		21: item(21, "comment", map[string]any{"text": "top", "kids": []int{22}}),
		22: item(22, "comment", map[string]any{"text": "nested"}),
		23: item(23, "comment", map[string]any{"dead": true}),
	}}
	c := f.server(t)
	tree, err := c.Thread(context.Background(), Item{ID: 20, Kids: []int{21, 23}})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 1 || tree[0].ID != 21 || len(tree[0].Children) != 1 || tree[0].Children[0].Text != "nested" {
		t.Fatalf("fallback tree wrong: %+v", tree)
	}
}

func TestResolve(t *testing.T) {
	f := &fakeHN{items: map[int]any{
		30: item(30, "story", map[string]any{"title": "s"}),
		31: item(31, "comment", map[string]any{"parent": 30}),
		32: item(32, "comment", map[string]any{"parent": 31}),
		40: item(40, "story", map[string]any{"dead": true}),
		41: item(41, "comment", map[string]any{"parent": 40}),
	}}
	c := f.server(t)
	ctx := context.Background()

	story, focus, err := c.Resolve(ctx, ItemRef{ID: 32})
	if err != nil || story.ID != 30 || focus != 32 {
		t.Errorf("comment link: story %d focus %d err %v", story.ID, focus, err)
	}
	story, focus, err = c.Resolve(ctx, ItemRef{ID: 30, Anchor: 31})
	if err != nil || story.ID != 30 || focus != 31 {
		t.Errorf("story#anchor link: story %d focus %d err %v", story.ID, focus, err)
	}
	if _, _, err := c.Resolve(ctx, ItemRef{ID: 41}); !errors.Is(err, ErrNotFound) {
		t.Errorf("dead story: err = %v", err)
	}
	if _, _, err := c.Resolve(ctx, ItemRef{ID: 999}); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing item: err = %v", err)
	}
}

func TestRetriesServerErrors(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte(`[7]`))
	}))
	defer srv.Close()
	c := NewClient()
	c.FirebaseURL = srv.URL
	ids, err := c.Feed(context.Background(), "top")
	if err != nil || len(ids) != 1 || calls.Load() != 3 {
		t.Fatalf("ids %v err %v calls %d", ids, err, calls.Load())
	}
}

func TestNoRetryOnClientError(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "no", http.StatusForbidden)
	}))
	defer srv.Close()
	c := NewClient()
	c.FirebaseURL = srv.URL
	_, err := c.Feed(context.Background(), "top")
	var se *StatusError
	if !errors.As(err, &se) || se.Code != 403 || calls.Load() != 1 {
		t.Fatalf("err %v calls %d", err, calls.Load())
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
