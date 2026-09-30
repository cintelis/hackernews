package hn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	firebaseURL = "https://hacker-news.firebaseio.com/v0"
	algoliaURL  = "https://hn.algolia.com/api/v1"

	itemTTL        = 5 * time.Minute
	maxInFlight    = 16 // concurrent item requests (feed pages, thread fallback)
	fallbackDepth  = 8  // reply depth fetched when Algolia is unavailable
	maxResolveHops = 64
	maxBody        = 32 << 20
)

// Client talks to both HN APIs. Items are cached for itemTTL; Purge and
// Invalidate drop them so a refresh really refetches.
type Client struct {
	HTTP        *http.Client
	FirebaseURL string
	AlgoliaURL  string

	mu    sync.Mutex
	items map[int]cachedItem
}

type cachedItem struct {
	item    Item
	expires time.Time
}

func NewClient() *Client {
	return &Client{
		HTTP:        &http.Client{Timeout: 15 * time.Second},
		FirebaseURL: firebaseURL,
		AlgoliaURL:  algoliaURL,
		items:       map[int]cachedItem{},
	}
}

// Feed returns the story ids of a feed: top, new, best, ask, show or job.
func (c *Client) Feed(ctx context.Context, feed string) ([]int, error) {
	var ids []int
	err := c.getJSON(ctx, fmt.Sprintf("%s/%sstories.json", c.FirebaseURL, feed), &ids)
	return ids, err
}

// Item fetches one item, from cache when fresh.
func (c *Client) Item(ctx context.Context, id int) (Item, error) {
	c.mu.Lock()
	ci, ok := c.items[id]
	c.mu.Unlock()
	if ok && time.Now().Before(ci.expires) {
		return ci.item, nil
	}
	var it *Item
	if err := c.getJSON(ctx, fmt.Sprintf("%s/item/%d.json", c.FirebaseURL, id), &it); err != nil {
		return Item{}, err
	}
	if it == nil { // the API answers 200 + null for ids that don't exist
		return Item{}, ErrNotFound
	}
	it.sanitize()
	c.mu.Lock()
	c.items[id] = cachedItem{item: *it, expires: time.Now().Add(itemTTL)}
	c.mu.Unlock()
	return *it, nil
}

// Items fetches many items concurrently, keeping input order and dropping
// failures and deleted or dead items.
func (c *Client) Items(ctx context.Context, ids []int) []Item {
	out := make([]*Item, len(ids))
	sem := make(chan struct{}, maxInFlight)
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			it, err := c.Item(ctx, id)
			<-sem
			if err == nil && !it.Deleted && !it.Dead {
				out[i] = &it
			}
		}()
	}
	wg.Wait()
	items := make([]Item, 0, len(ids))
	for _, it := range out {
		if it != nil {
			items = append(items, *it)
		}
	}
	return items
}

// Invalidate drops items from the cache.
func (c *Client) Invalidate(ids ...int) {
	c.mu.Lock()
	for _, id := range ids {
		delete(c.items, id)
	}
	c.mu.Unlock()
}

// Purge empties the cache.
func (c *Client) Purge() {
	c.mu.Lock()
	c.items = map[int]cachedItem{}
	c.mu.Unlock()
}

// Thread loads a story's whole comment tree. Algolia returns it in a single
// request; if that fails, the tree is walked through the official API.
// Top-level comments follow HN's ranking (the story's kids order).
func (c *Client) Thread(ctx context.Context, story Item) ([]*Comment, error) {
	tree, err := c.algoliaThread(ctx, story.ID)
	if err == nil {
		return rankTopLevel(tree, story.Kids), nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return c.firebaseThread(ctx, story.Kids)
}

type algoliaItem struct {
	ID         int           `json:"id"`
	Type       string        `json:"type"`
	Author     *string       `json:"author"`
	Text       *string       `json:"text"`
	CreatedAtI int64         `json:"created_at_i"`
	Children   []algoliaItem `json:"children"`
}

func (c *Client) algoliaThread(ctx context.Context, storyID int) ([]*Comment, error) {
	var root algoliaItem
	if err := c.getJSON(ctx, fmt.Sprintf("%s/items/%d", c.AlgoliaURL, storyID), &root); err != nil {
		return nil, err
	}
	return convertAlgolia(root.Children), nil
}

func convertAlgolia(items []algoliaItem) []*Comment {
	var out []*Comment
	for _, a := range items {
		children := convertAlgolia(a.Children)
		deleted := a.Author == nil || *a.Author == ""
		if deleted && len(children) == 0 {
			continue
		}
		cm := &Comment{ID: a.ID, Time: a.CreatedAtI, Deleted: deleted, Children: children}
		if !deleted {
			cm.By = Sanitize(*a.Author)
			if a.Text != nil {
				cm.Text, cm.Links = RenderHTML(*a.Text)
			}
		}
		out = append(out, cm)
	}
	return out
}

// rankTopLevel orders comments by HN's ranking; any the ranking doesn't
// know about (newer than the cached story) go last, in their given order.
func rankTopLevel(tree []*Comment, kids []int) []*Comment {
	byID := make(map[int]*Comment, len(tree))
	for _, cm := range tree {
		byID[cm.ID] = cm
	}
	out := make([]*Comment, 0, len(tree))
	for _, id := range kids {
		if cm, ok := byID[id]; ok {
			out = append(out, cm)
			delete(byID, id)
		}
	}
	for _, cm := range tree {
		if _, left := byID[cm.ID]; left {
			out = append(out, cm)
		}
	}
	return out
}

func (c *Client) firebaseThread(ctx context.Context, kids []int) ([]*Comment, error) {
	sem := make(chan struct{}, maxInFlight)
	var errMu sync.Mutex
	var firstErr error

	var load func(ids []int, depth int) []*Comment
	load = func(ids []int, depth int) []*Comment {
		out := make([]*Comment, len(ids))
		var wg sync.WaitGroup
		for i, id := range ids {
			wg.Add(1)
			go func() {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
				case <-ctx.Done():
					return
				}
				it, err := c.Item(ctx, id)
				<-sem // released before recursing, so children can't starve
				if err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					errMu.Unlock()
					return
				}
				var children []*Comment
				if depth < fallbackDepth {
					children = load(it.Kids, depth+1)
				}
				gone := it.Deleted || it.Dead
				if gone && len(children) == 0 {
					return
				}
				cm := &Comment{ID: it.ID, Time: it.Time, Deleted: gone, Children: children}
				if !gone {
					cm.By = it.By
					cm.Text, cm.Links = RenderHTML(it.Text)
				}
				out[i] = cm
			}()
		}
		wg.Wait()
		res := out[:0]
		for _, cm := range out {
			if cm != nil {
				res = append(res, cm)
			}
		}
		return res
	}

	tree := load(kids, 0)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(tree) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return tree, nil
}

// Resolve finds the story an item link points into: a story id resolves to
// itself; a comment id walks up its parents. focus is the comment to land on.
func (c *Client) Resolve(ctx context.Context, ref ItemRef) (story Item, focus int, err error) {
	cur, err := c.Item(ctx, ref.ID)
	if err != nil {
		return Item{}, 0, err
	}
	if cur.IsComment() {
		focus = cur.ID
	}
	for hops := 0; cur.IsComment(); hops++ {
		if cur.Parent == 0 || hops >= maxResolveHops {
			return Item{}, 0, ErrNotFound
		}
		if cur, err = c.Item(ctx, cur.Parent); err != nil {
			return Item{}, 0, err
		}
	}
	if cur.Deleted || cur.Dead {
		return Item{}, 0, ErrNotFound
	}
	if ref.Anchor > 0 {
		focus = ref.Anchor
	}
	return cur, focus, nil
}

// getJSON GETs and decodes, retrying twice with backoff on network errors,
// 429 and 5xx. A 404 is ErrNotFound.
func (c *Client) getJSON(ctx context.Context, url string, v any) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(250<<(attempt-1)) * time.Millisecond):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if err = c.getOnce(ctx, url, v); err == nil || !retryable(ctx, err) {
			break
		}
	}
	var se *StatusError
	if errors.As(err, &se) && se.Code == http.StatusNotFound {
		return ErrNotFound
	}
	return err
}

func (c *Client) getOnce(ctx context.Context, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "cintelis (+https://github.com/cintelis/hackernews)")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
		return &StatusError{Code: res.StatusCode, URL: url}
	}
	return json.NewDecoder(io.LimitReader(res.Body, maxBody)).Decode(v)
}

func retryable(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}
	var se *StatusError
	if errors.As(err, &se) {
		return se.Code == http.StatusTooManyRequests || se.Code >= 500
	}
	var syn *json.SyntaxError
	var typ *json.UnmarshalTypeError
	return !errors.As(err, &syn) && !errors.As(err, &typ)
}
