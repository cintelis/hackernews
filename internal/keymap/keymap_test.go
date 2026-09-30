package keymap

import (
	"testing"
	"time"

	"github.com/cintelis/hackernews/internal/app"
)

var modes = []app.Mode{app.ModeList, app.ModeDetail, app.ModeError, app.ModeLinks, app.ModeHelp, app.ModeSearch}

// A key bound twice in one mode would silently shadow the later binding.
func TestNoConflicts(t *testing.T) {
	for _, m := range modes {
		seen := map[string]app.Command{}
		for _, b := range Bindings {
			if !has(b.Modes, m) {
				continue
			}
			for _, k := range b.Keys {
				if prev, dup := seen[k]; dup {
					t.Errorf("mode %d: key %q bound to both %d and %d", m, k, prev, b.Cmd)
				}
				seen[k] = b.Cmd
			}
		}
	}
}

func TestResolve(t *testing.T) {
	var r Resolver
	now := time.Now()
	if c := r.Resolve(app.ModeList, "j", now); c != app.CmdDown {
		t.Errorf("j = %d", c)
	}
	if c := r.Resolve(app.ModeList, "h", now); c != app.CmdPrevCategory {
		t.Errorf("h in list = %d", c)
	}
	if c := r.Resolve(app.ModeDetail, "h", now); c != app.CmdBack {
		t.Errorf("h in detail = %d", c)
	}
	if c := r.Resolve(app.ModeDetail, "enter", now); c != app.CmdLinks {
		t.Errorf("enter in detail = %d", c)
	}
	if c := r.Resolve(app.ModeLinks, "s", now); c != app.CmdNone {
		t.Errorf("s in links popup should do nothing, got %d", c)
	}
}

func TestSequence(t *testing.T) {
	var r Resolver
	now := time.Now()
	if c := r.Resolve(app.ModeList, "g", now); c != app.CmdNone {
		t.Fatalf("first g = %d", c)
	}
	if c := r.Resolve(app.ModeList, "g", now.Add(300*time.Millisecond)); c != app.CmdTop {
		t.Fatalf("g g = %d", c)
	}
	r.Resolve(app.ModeList, "g", now)
	if c := r.Resolve(app.ModeList, "g", now.Add(2*time.Second)); c != app.CmdNone {
		t.Fatalf("slow g g should not fire, got %d", c)
	}
	r = Resolver{}
	r.Resolve(app.ModeList, "g", now)
	if c := r.Resolve(app.ModeList, "j", now); c != app.CmdDown {
		t.Fatalf("g then j should still move, got %d", c)
	}
}

func TestHelpAndHints(t *testing.T) {
	for _, m := range []app.Mode{app.ModeList, app.ModeDetail, app.ModeLinks} {
		if len(Help(m)) == 0 {
			t.Errorf("mode %d has no help", m)
		}
		if Hints(m) == "" {
			t.Errorf("mode %d has no hints", m)
		}
	}
}
