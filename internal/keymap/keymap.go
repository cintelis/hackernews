// Package keymap is every key binding as data. Key handling, the help
// screen and the status-bar hints are all generated from Bindings, so they
// can't drift apart.
package keymap

import (
	"strings"
	"time"

	"github.com/cintelis/hackernews/internal/app"
)

// Binding maps keys to a command in some modes. Keys are Bubble Tea key
// names; a space-separated key ("g g") is a two-key sequence.
type Binding struct {
	Modes []app.Mode
	Keys  []string
	Cmd   app.Command
	Label string // key names as shown in help, e.g. "j k ↑ ↓"
	Help  string // "" keeps the binding out of the help screen
	Hint  string // short status-bar hint, "" for none
}

var (
	browse = []app.Mode{app.ModeList, app.ModeDetail, app.ModeError}
	list   = []app.Mode{app.ModeList}
	detail = []app.Mode{app.ModeDetail}
	moving = []app.Mode{app.ModeList, app.ModeDetail, app.ModeLinks}
	all    = []app.Mode{app.ModeList, app.ModeDetail, app.ModeError, app.ModeLinks, app.ModeHelp}
	search = []app.Mode{app.ModeSearch}
	every  = []app.Mode{app.ModeList, app.ModeDetail, app.ModeError, app.ModeLinks, app.ModeHelp, app.ModeSearch}
)

var Bindings = []Binding{
	{all, []string{"q"}, app.CmdQuit, "q", "quit", "q quit"}, // not while typing a search
	{every, []string{"ctrl+c"}, app.CmdQuit, "", "", ""},
	{moving, []string{"j", "down"}, app.CmdDown, "j k ↑ ↓", "down / up", "j/k move"},
	{moving, []string{"k", "up"}, app.CmdUp, "", "", ""},
	{moving, []string{"g g", "home"}, app.CmdTop, "g g / G", "first / last", ""},
	{moving, []string{"G", "end"}, app.CmdBottom, "", "", ""},
	{moving, []string{"ctrl+d", "pgdown"}, app.CmdHalfDown, "ctrl+d / ctrl+u", "half page down / up", ""},
	{moving, []string{"ctrl+u", "pgup"}, app.CmdHalfUp, "", "", ""},

	{list, []string{"l", "right", "tab"}, app.CmdNextCategory, "h l / tab", "previous / next tab", ""},
	{list, []string{"h", "left", "shift+tab"}, app.CmdPrevCategory, "", "", ""},
	{list, []string{"1"}, app.CmdCategory1, "1 – 6", "go to feed 1–6", ""},
	{list, []string{"2"}, app.CmdCategory2, "", "", ""},
	{list, []string{"3"}, app.CmdCategory3, "", "", ""},
	{list, []string{"4"}, app.CmdCategory4, "", "", ""},
	{list, []string{"5"}, app.CmdCategory5, "", "", ""},
	{list, []string{"6"}, app.CmdCategory6, "", "", ""},
	{list, []string{"enter", "c"}, app.CmdOpen, "⏎ / c", "read the thread", "⏎ open"},
	{list, []string{"x"}, app.CmdClearHistory, "x", "forget history (History tab)", ""},

	{detail, []string{" "}, app.CmdCollapse, "space", "fold / unfold replies", "space fold"},
	{detail, []string{"n"}, app.CmdSortNewest, "n", "newest comments first / ranked", "n newest"},
	{detail, []string{"enter"}, app.CmdLinks, "⏎", "list this comment's links", "⏎ links"},

	{browse, []string{"o"}, app.CmdOpenURL, "o", "open the story's link", ""},
	{browse, []string{"y"}, app.CmdOpenHN, "y", "open on news.ycombinator.com", ""},
	{[]app.Mode{app.ModeList, app.ModeDetail}, []string{"s"}, app.CmdToggleSave, "s", "save / unsave", "s save"},
	{browse, []string{"r"}, app.CmdRefresh, "r", "reload", ""},
	{browse, []string{"S"}, app.CmdSavedView, "S", "saved stories", ""},
	{browse, []string{"H"}, app.CmdHistoryView, "H", "reading history", ""},
	{browse, []string{"t"}, app.CmdTheme, "t", "switch theme", ""},
	{[]app.Mode{app.ModeDetail, app.ModeError}, []string{"esc", "backspace", "h", "left"}, app.CmdBack, "h / esc", "back", "h/esc back"},
	{browse, []string{"/"}, app.CmdSearch, "/", "search all of Hacker News", "/ search"},
	{browse, []string{"?"}, app.CmdHelp, "?", "show keys", ""},

	// while typing in the search box every other key is text
	{search, []string{"enter", "down"}, app.CmdOpen, "⏎ / ↓", "go to the results", "⏎ results"},
	{search, []string{"backspace"}, app.CmdDeleteChar, "backspace", "delete a character", ""},
	{search, []string{"ctrl+u"}, app.CmdClearInput, "ctrl+u", "clear the search", "ctrl+u clear"},
	{search, []string{"esc"}, app.CmdBack, "esc", "stop typing", "esc done"},

	{[]app.Mode{app.ModeLinks}, []string{"enter"}, app.CmdOpen, "⏎", "open link", "⏎ open"},
	{[]app.Mode{app.ModeLinks}, []string{"o"}, app.CmdOpenURL, "o", "open in browser", "o browser"},
	{[]app.Mode{app.ModeLinks}, []string{"esc", "backspace"}, app.CmdBack, "esc", "close", "esc close"},

	{[]app.Mode{app.ModeHelp}, []string{"esc", "backspace", "?"}, app.CmdBack, "esc", "close", "esc close"},
}

// sequenceTimeout: the second key of a sequence must follow within this.
const sequenceTimeout = time.Second

// Resolver turns key presses into commands, tracking a pending sequence prefix.
type Resolver struct {
	pending string
	at      time.Time
}

// Resolve returns the command for key in mode, or CmdNone.
func (r *Resolver) Resolve(mode app.Mode, key string, now time.Time) app.Command {
	if r.pending != "" && now.Sub(r.at) <= sequenceTimeout {
		seq := r.pending + " " + key
		r.pending = ""
		if c := lookup(mode, seq); c != app.CmdNone {
			return c
		}
	}
	r.pending = ""
	if c := lookup(mode, key); c != app.CmdNone {
		return c
	}
	if startsSequence(mode, key) {
		r.pending, r.at = key, now
	}
	return app.CmdNone
}

func lookup(mode app.Mode, key string) app.Command {
	for _, b := range Bindings {
		if has(b.Modes, mode) && has(b.Keys, key) {
			return b.Cmd
		}
	}
	return app.CmdNone
}

func startsSequence(mode app.Mode, key string) bool {
	for _, b := range Bindings {
		if !has(b.Modes, mode) {
			continue
		}
		for _, k := range b.Keys {
			if strings.HasPrefix(k, key+" ") {
				return true
			}
		}
	}
	return false
}

// HelpEntry is one line of the help screen.
type HelpEntry struct{ Keys, Desc string }

// Help lists the documented bindings for a mode, in declaration order.
func Help(mode app.Mode) []HelpEntry {
	var out []HelpEntry
	for _, b := range Bindings {
		if b.Help != "" && has(b.Modes, mode) {
			out = append(out, HelpEntry{b.Label, b.Help})
		}
	}
	return out
}

// Hints is the status-bar line for a mode.
func Hints(mode app.Mode) string {
	var parts []string
	for _, b := range Bindings {
		if b.Hint != "" && has(b.Modes, mode) && b.Cmd != app.CmdQuit {
			parts = append(parts, b.Hint)
		}
	}
	quit := "q quit"
	if mode == app.ModeSearch {
		quit = "ctrl+c quit"
	}
	return strings.Join(append(parts, quit), " · ")
}

func has[T comparable](xs []T, x T) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
