package hn

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ItemRef is a link into HN itself: /item?id=ID or /item?id=ID#ANCHOR.
// ID may be a story or a comment.
type ItemRef struct {
	ID     int
	Anchor int
}

// ParseItemLink recognizes news.ycombinator.com/item links, which the app
// opens in place instead of in the browser.
func ParseItemLink(raw string) (ItemRef, bool) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ItemRef{}, false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if host != "news.ycombinator.com" || u.Path != "/item" {
		return ItemRef{}, false
	}
	id, err := strconv.Atoi(u.Query().Get("id"))
	if err != nil || id <= 0 {
		return ItemRef{}, false
	}
	ref := ItemRef{ID: id}
	if a, err := strconv.Atoi(u.Fragment); err == nil && a > 0 {
		ref.Anchor = a
	}
	return ref, true
}

// ItemURL is the HN web page for an item.
func ItemURL(id int) string { return "https://news.ycombinator.com/item?id=" + strconv.Itoa(id) }

// ReplyURL is HN's reply form for a comment; after posting, HN returns to
// that comment in its thread.
func ReplyURL(storyID, commentID int) string {
	back := fmt.Sprintf("item?id=%d#%d", storyID, commentID)
	return fmt.Sprintf("https://news.ycombinator.com/reply?id=%d&goto=%s", commentID, url.QueryEscape(back))
}
