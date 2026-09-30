// Package hn is the Hacker News client: story feeds and items from the
// official Firebase API, whole comment threads from the Algolia HN API.
// Both are public, read-only and need no account.
package hn

import (
	"errors"
	"fmt"
)

// Item is a story, job, poll, comment or poll option, as the official API
// returns it. All string fields are sanitized before an Item leaves this
// package; Text is still HTML (see RenderHTML).
type Item struct {
	ID          int    `json:"id"`
	Type        string `json:"type"`
	By          string `json:"by"`
	Time        int64  `json:"time"`
	Text        string `json:"text"`
	Dead        bool   `json:"dead"`
	Deleted     bool   `json:"deleted"`
	Parent      int    `json:"parent"`
	Kids        []int  `json:"kids"`
	URL         string `json:"url"`
	Score       int    `json:"score"`
	Title       string `json:"title"`
	Descendants int    `json:"descendants"`
}

func (i Item) IsComment() bool { return i.Type == "comment" || i.Type == "pollopt" }

func (i *Item) sanitize() {
	i.By = Sanitize(i.By)
	i.Title = Sanitize(i.Title)
	i.URL = Sanitize(i.URL)
}

// Comment is one node of a rendered thread: Text is already plain text,
// with its links pulled out into Links.
type Comment struct {
	ID       int
	By       string
	Time     int64
	Text     string
	Links    []Link
	Deleted  bool // kept only as a placeholder when it has live replies
	Children []*Comment
}

// ErrNotFound: the item doesn't exist, or was deleted.
var ErrNotFound = errors.New("item not found")

// StatusError is a non-200 HTTP response.
type StatusError struct {
	Code int
	URL  string
}

func (e *StatusError) Error() string { return fmt.Sprintf("HTTP %d from %s", e.Code, e.URL) }
