//go:build live

// Live checks against the real APIs, to catch format drift:
//
//	go test -tags live ./internal/hn/
package hn

import (
	"context"
	"testing"
	"time"
)

func TestLive(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := NewClient()

	ids, err := c.Feed(ctx, "top")
	if err != nil || len(ids) < 100 {
		t.Fatalf("top feed: %d ids, %v", len(ids), err)
	}
	items := c.Items(ctx, ids[:10])
	if len(items) < 8 {
		t.Fatalf("only %d of 10 top stories loaded", len(items))
	}

	var story Item
	for _, it := range items {
		if it.Descendants > 20 {
			story = it
			break
		}
	}
	if story.ID == 0 {
		t.Skip("no top story with comments right now")
	}
	algolia, err := c.algoliaThread(ctx, story.ID)
	if err != nil || len(algolia) == 0 {
		t.Fatalf("algolia thread for %d: %d comments, %v", story.ID, len(algolia), err)
	}
	fallback, err := c.firebaseThread(ctx, story.Kids)
	if err != nil || len(fallback) == 0 {
		t.Fatalf("firebase thread: %d, %v", len(fallback), err)
	}
	t.Logf("story %d %q: %d comments reported; algolia top-level %d, firebase top-level %d",
		story.ID, story.Title, story.Descendants, len(algolia), len(fallback))

	// pg's first post, and a known comment on it, resolve in-app
	s, focus, err := c.Resolve(ctx, ItemRef{ID: 15})
	if err != nil || s.ID != 1 || focus != 15 {
		t.Fatalf("resolve comment 15: story %d focus %d err %v", s.ID, focus, err)
	}
	if _, _, err := c.Resolve(ctx, ItemRef{ID: 999999999999}); err != ErrNotFound {
		t.Fatalf("missing item: %v", err)
	}
}

func TestLiveSearch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := NewClient()
	for _, tc := range []struct {
		q    string
		kind SearchKind
	}{
		{"kubernets", SearchMatched},                        // typo
		{"kube", SearchMatched},                             // prefix
		{"rust borrow checker zebra banana", SearchAnyWord}, // no story has every word
		{"sqlite wal mode performence", SearchAnyWord},      // typo + a word no title has
	} {
		items, kind, err := c.Search(ctx, tc.q, SearchOptions{})
		if err != nil || kind != tc.kind || len(items) == 0 {
			t.Fatalf("%q: %d items, kind %v, err %v", tc.q, len(items), kind, err)
		}
		t.Logf("%q (kind %d): %q", tc.q, kind, items[0].Title)
	}
}

func TestLiveSearchFilters(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := NewClient()
	yearAgo := time.Now().Add(-365 * 24 * time.Hour).Unix()
	for name, tc := range map[string]struct {
		q   string
		opt SearchOptions
	}{
		"rust, stories by date, past year":           {"rust", SearchOptions{ByDate: true, Since: yearAgo}},
		"kubernets (typo), by date, all time":        {"kubernets", SearchOptions{ByDate: true}},
		"no words, Show HN by popularity, past year": {"", SearchOptions{Tag: "show_hn", Since: yearAgo}},
		"no words, jobs by date":                     {"", SearchOptions{Tag: "job", ByDate: true}},
	} {
		items, kind, err := c.Search(ctx, tc.q, tc.opt)
		if err != nil || len(items) == 0 {
			t.Fatalf("%s: %d items, err %v", name, len(items), err)
		}
		for i := 1; tc.opt.ByDate && i < len(items); i++ {
			if items[i].Time > items[i-1].Time {
				t.Errorf("%s: not newest first at %d", name, i)
				break
			}
		}
		for _, it := range items {
			if tc.opt.Since > 0 && it.Time <= tc.opt.Since {
				t.Errorf("%s: %q is older than the range", name, it.Title)
				break
			}
		}
		t.Logf("%s (kind %d): %q", name, kind, items[0].Title)
	}
}
