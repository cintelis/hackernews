// Package app is cintelis's behavior as a pure state machine: State plus
// Update(action) → effects. It does no I/O — fetching, opening the browser
// and saving files are Effects that the UI layer runs, and their results come
// back as Actions. That keeps every navigation rule unit-testable.
package app

import (
	"github.com/cintelis/hackernews/internal/hn"
	"github.com/cintelis/hackernews/internal/store"
)

// Category is a tab: the six HN feeds, the local History and Saved lists, and Search.
type Category int

const (
	CatTop Category = iota
	CatNew
	CatBest
	CatAsk
	CatShow
	CatJobs
	CatHistory
	CatSaved
	CatSearch
)

var Categories = []Category{CatTop, CatNew, CatBest, CatAsk, CatShow, CatJobs, CatHistory, CatSaved, CatSearch}

var (
	categoryLabels = [...]string{"Top", "New", "Best", "Ask", "Show", "Jobs", "History", "Saved", "Search"}
	categoryFeeds  = [...]string{"top", "new", "best", "ask", "show", "job"}
)

func (c Category) Label() string { return categoryLabels[c] }
func (c Category) IsFeed() bool  { return c <= CatJobs }

// Feed is the API feed name, or "" for the local lists.
func (c Category) Feed() string {
	if c.IsFeed() {
		return categoryFeeds[c]
	}
	return ""
}

// Mode decides which key bindings apply.
type Mode int

const (
	ModeList Mode = iota
	ModeDetail
	ModeError
	ModeLinks
	ModeHelp
	ModeSearch // typing a search query
)

// Command is a user intent, produced by the keymap.
type Command int

const (
	CmdNone Command = iota
	CmdDown
	CmdUp
	CmdTop
	CmdBottom
	CmdHalfDown
	CmdHalfUp
	CmdNextCategory
	CmdPrevCategory
	CmdCategory1 // …through CmdCategory1+5: jump to feed 1–6
	CmdCategory2
	CmdCategory3
	CmdCategory4
	CmdCategory5
	CmdCategory6
	CmdOpen
	CmdOpenURL
	CmdOpenHN
	CmdToggleSave
	CmdRefresh
	CmdClearHistory
	CmdBack
	CmdCollapse
	CmdLinks
	CmdSavedView
	CmdHistoryView
	CmdHelp
	CmdTheme
	CmdQuit
	CmdSearch      // open search and start typing
	CmdDeleteChar  // backspace in the search box
	CmdClearInput  // empty the search box
	CmdSortNewest  // switch a thread between ranked and newest-first
	CmdSearchType  // cycle the search filters: type,
	CmdSearchOrder // order,
	CmdSearchRange // and time range
	CmdReply       // reply to the comment, in the browser
	CmdCopyLink    // copy the story's or comment's link
	CmdCopyText    // copy the story's title or the comment's text
	CmdToggleMouse // let the terminal have the mouse, for selecting text
)

// Action is anything Update accepts: a Command or one of the result types below.
type Action any

// Results of effects, and other outside events.
type (
	FeedLoaded struct {
		Gen int
		IDs []int
		Err error
	}
	ItemsLoaded struct {
		Gen   int
		Items []hn.Item
	}
	ThreadLoaded struct {
		Gen   int
		Story *hn.Item // refreshed story, when the load refetched it
		Tree  []*hn.Comment
		Err   error
	}
	Resolved struct {
		Gen   int
		Story hn.Item
		Focus int
		Err   error
	}
	UpdateFound  struct{ Version string }
	Resized      struct{ ListPage, DetailPage int }
	Flash        struct{ Text string }
	Typed        struct{ Text string } // characters typed into the search box
	SearchLoaded struct {
		Gen   int
		Query string
		Items []hn.Item
		Kind  hn.SearchKind
		Err   error
	}
)

// Effect is work for the UI layer to perform.
type Effect any

type (
	FetchFeed struct {
		Gen   int
		Feed  string
		Purge bool // drop cached items first (refresh)
	}
	FetchItems struct {
		Gen   int
		IDs   []int
		Purge bool
	}
	FetchThread struct {
		Gen     int
		Story   hn.Item
		Refresh bool // also refetch the story itself
	}
	ResolveLink struct {
		Gen int
		Ref hn.ItemRef
	}
	OpenURL     struct{ URL string }
	CopyText    struct{ Text, What string } // What: for the confirmation, e.g. "link"
	SetMouse    struct{ On bool }
	SaveSaved   struct{ Entries []store.Entry }
	SaveHistory struct{ Entries []store.Entry }
	Quit        struct{}
	RunSearch   struct {
		Gen     int
		Query   string
		Options hn.SearchOptions
	}
)
