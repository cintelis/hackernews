package app

import (
	"errors"
	"sort"
	"time"

	"github.com/cintelis/hackernews/internal/hn"
	"github.com/cintelis/hackernews/internal/store"
)

// PageSize is how many stories are fetched per page of a list.
const PageSize = 30

// prefetchMargin: the next page is requested when the cursor gets this close
// to the end of what's loaded.
const prefetchMargin = 5

// ErrNoStories: every story on the page failed to load.
var ErrNoStories = errors.New("couldn't load stories")

type Screen int

const (
	ScreenList Screen = iota
	ScreenDetail
	ScreenError // an in-app link that couldn't be opened
)

// List is the story list of the current category, loaded a page at a time.
type List struct {
	IDs       []int
	Items     []hn.Item
	Requested int // IDs[:Requested] have been asked for
	Cursor    int
	Loading   bool
	Err       error
	gen       int
}

// HasMore reports whether further pages exist.
func (l *List) HasMore() bool { return l.Requested < len(l.IDs) }

// Detail is one open story and its comments.
type Detail struct {
	Story     hn.Item
	Tree      []*hn.Comment
	Flat      []Flat
	Cursor    int
	Collapsed map[int]bool
	Loading   bool
	Err       error
	focus     int  // comment to land on once the thread loads
	newest    bool // Flat is in newest-first order
	gen       int
}

// Current is the comment under the cursor, or nil.
func (d *Detail) Current() *hn.Comment {
	if d.Cursor >= 0 && d.Cursor < len(d.Flat) {
		return d.Flat[d.Cursor].Comment
	}
	return nil
}

func (d *Detail) reflatten(newest bool) {
	d.newest = newest
	if newest {
		d.Flat = FlattenNewest(d.Tree)
	} else {
		d.Flat = Flatten(d.Tree, d.Collapsed)
	}
}

// resort switches order, keeping the cursor on the same comment when it's
// still visible.
func (d *Detail) resort(newest bool) {
	cur := d.Current()
	d.reflatten(newest)
	d.Cursor = 0
	if cur != nil {
		if i := d.indexOf(cur.ID); i >= 0 {
			d.Cursor = i
		}
	}
}

func (d *Detail) indexOf(id int) int {
	for i, f := range d.Flat {
		if f.Comment.ID == id {
			return i
		}
	}
	return -1
}

// Search is the search box and what the shown results were found by.
type Search struct {
	Query   string
	Editing bool          // keys go to the box, not the keymap
	Shown   string        // the query the listed results belong to
	Kind    hn.SearchKind // how they were found
}

type LinksPopup struct {
	Links  []hn.Link
	Cursor int
}

// State is the whole app.
type State struct {
	Category Category
	List     List
	Screen   Screen
	Detail   *Detail
	Stack    []*Detail // details left behind by following in-app links

	Resolving  bool
	ResolveRef hn.ItemRef
	ResolveErr error

	Links  *LinksPopup
	Newest bool // threads list comments newest first (a session-wide switch)
	Help   bool
	Search Search
	Light  bool

	Saved   []store.Entry
	History []store.Entry

	UpdateAvailable string
	Flash           string

	ListPage   int // story rows that fit on screen
	DetailPage int // rough number of comments that fit on screen

	Now func() time.Time

	prevCategory Category // where esc leaves the Search tab to

	savedSet   map[int]bool
	viewedSet  map[int]bool
	nextGen    int
	resolveGen int
}

// New builds the initial state; Start returns the effects that load it.
func New(saved, history []store.Entry) *State {
	s := &State{Saved: saved, History: history, Now: time.Now, ListPage: 10, DetailPage: 4}
	s.reindex()
	return s
}

// Start loads the first feed.
func (s *State) Start() []Effect { return s.loadList(false) }

func (s *State) Mode() Mode {
	switch {
	case s.Help:
		return ModeHelp
	case s.Links != nil:
		return ModeLinks
	case s.Screen == ScreenList && s.Category == CatSearch && s.Search.Editing:
		return ModeSearch
	case s.Screen == ScreenDetail:
		return ModeDetail
	case s.Screen == ScreenError:
		return ModeError
	}
	return ModeList
}

func (s *State) IsSaved(id int) bool  { return s.savedSet[id] }
func (s *State) IsViewed(id int) bool { return s.viewedSet[id] }

// Busy reports whether anything the user is looking at is loading.
func (s *State) Busy() bool {
	if s.Resolving {
		return true
	}
	if s.Screen == ScreenDetail && s.Detail != nil {
		return s.Detail.Loading
	}
	return s.Screen == ScreenList && s.List.Loading
}

func (s *State) reindex() {
	s.savedSet = make(map[int]bool, len(s.Saved))
	for _, e := range s.Saved {
		s.savedSet[e.ID] = true
	}
	s.viewedSet = make(map[int]bool, len(s.History))
	for _, e := range s.History {
		s.viewedSet[e.ID] = true
	}
}

func (s *State) gen() int {
	s.nextGen++
	return s.nextGen
}

// Flat is one visible row of a comment tree.
type Flat struct {
	Comment *hn.Comment
	Depth   int
	Hidden  int    // replies hidden because this comment is collapsed
	ReplyTo string // newest-first only: who this answers ("" for top-level)
}

// FlattenNewest lists every comment newest first, as one flat list: the
// thread's latest activity on top, wherever in the tree it happened.
func FlattenNewest(tree []*hn.Comment) []Flat {
	var out []Flat
	var walk func(nodes []*hn.Comment, parent *hn.Comment)
	walk = func(nodes []*hn.Comment, parent *hn.Comment) {
		for _, c := range nodes {
			if !c.Deleted { // deleted placeholders only hold the tree's shape
				f := Flat{Comment: c}
				if parent != nil {
					f.ReplyTo = parent.By
					if parent.Deleted {
						f.ReplyTo = "[deleted]"
					}
				}
				out = append(out, f)
			}
			walk(c.Children, c)
		}
	}
	walk(tree, nil)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Comment, out[j].Comment
		if a.Time != b.Time {
			return a.Time > b.Time
		}
		return a.ID > b.ID // ids grow over time: a tiebreak that stays newest-first
	})
	return out
}

// Flatten lists the visible comments depth-first; a collapsed comment's
// replies are skipped and counted instead.
func Flatten(tree []*hn.Comment, collapsed map[int]bool) []Flat {
	var out []Flat
	var walk func([]*hn.Comment, int)
	walk = func(nodes []*hn.Comment, depth int) {
		for _, c := range nodes {
			if collapsed[c.ID] {
				out = append(out, Flat{Comment: c, Depth: depth, Hidden: countReplies(c)})
				continue
			}
			out = append(out, Flat{Comment: c, Depth: depth})
			walk(c.Children, depth+1)
		}
	}
	walk(tree, 0)
	return out
}

func countReplies(c *hn.Comment) int {
	n := len(c.Children)
	for _, ch := range c.Children {
		n += countReplies(ch)
	}
	return n
}
