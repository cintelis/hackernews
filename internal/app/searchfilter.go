package app

import (
	"time"

	"github.com/cintelis/hackernews/internal/hn"
)

// The search filters, after hn.algolia.com's "Search [type] by [order] for [range]".

type SearchType int

const (
	TypeStories SearchType = iota
	TypeAsk
	TypeShow
	TypeLaunch
	TypeJobs
	TypePolls
	searchTypes
)

var (
	searchTypeLabels = [...]string{"Stories", "Ask HN", "Show HN", "Launch HN", "Jobs", "Polls"}
	searchTypeTags   = [...]string{"story", "ask_hn", "show_hn", "launch_hn", "job", "poll"}
)

func (t SearchType) Label() string { return searchTypeLabels[t] }

type SearchOrder int

const (
	ByPopularity SearchOrder = iota
	ByDate
	searchOrders
)

func (o SearchOrder) Label() string {
	if o == ByDate {
		return "Date"
	}
	return "Popularity"
}

type SearchRange int

const (
	AllTime SearchRange = iota
	Past24h
	PastWeek
	PastMonth
	PastYear
	searchRanges
)

var (
	searchRangeLabels = [...]string{"All time", "Last 24h", "Past week", "Past month", "Past year"}
	searchRangeSpans  = [...]time.Duration{0, 24 * time.Hour, 7 * 24 * time.Hour, 30 * 24 * time.Hour, 365 * 24 * time.Hour}
)

func (r SearchRange) Label() string { return searchRangeLabels[r] }

// DefaultSearch is where a search starts: the newest stories from the past year.
var DefaultSearch = Search{Type: TypeStories, Order: ByDate, Range: PastYear}

// Filtered reports whether any filter differs from its default. A filtered
// search runs even with no words: "Show HN, past week, by popularity" browses.
func (s Search) Filtered() bool {
	d := DefaultSearch
	return s.Type != d.Type || s.Order != d.Order || s.Range != d.Range
}

func (s Search) options(now time.Time) hn.SearchOptions {
	opt := hn.SearchOptions{Tag: searchTypeTags[s.Type], ByDate: s.Order == ByDate}
	if span := searchRangeSpans[s.Range]; span > 0 {
		opt.Since = now.Add(-span).Unix()
	}
	return opt
}

// key identifies what a search asks for, to skip re-running an identical one.
func (s Search) key(q string) string {
	return q + "\x00" + string(rune('0'+s.Type)) + string(rune('0'+s.Order)) + string(rune('0'+s.Range))
}

func (s *State) cycleSearchFilter(c Command) []Effect {
	switch c {
	case CmdSearchType:
		s.Search.Type = (s.Search.Type + 1) % searchTypes
	case CmdSearchOrder:
		s.Search.Order = (s.Search.Order + 1) % searchOrders
	case CmdSearchRange:
		s.Search.Range = (s.Search.Range + 1) % searchRanges
	}
	return s.searchChanged()
}
