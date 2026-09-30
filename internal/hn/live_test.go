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
