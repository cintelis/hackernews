package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cintelis/hackernews/internal/app"
	"github.com/cintelis/hackernews/internal/hn"
	"github.com/cintelis/hackernews/internal/store"
)

// checkFrame asserts the frame fills the terminal exactly.
func checkFrame(t *testing.T, m *Model) string {
	t.Helper()
	out := m.View()
	lines := strings.Split(out, "\n")
	if len(lines) != m.height {
		t.Fatalf("frame has %d lines, terminal has %d", len(lines), m.height)
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > m.width {
			t.Fatalf("line %d is %d wide (terminal %d): %q", i, w, m.width, l)
		}
	}
	return out
}

func newModel(t *testing.T, w, h int) *Model {
	dir := t.TempDir()
	s := app.New(nil, nil)
	m := New(s, hn.NewClient(), store.Saved(dir), store.History(dir), "dev")
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

func deepTree(depth int) []*hn.Comment {
	root := &hn.Comment{ID: 1000, By: "a", Text: strings.Repeat("long words wrap ", 30)}
	cur := root
	for i := 1; i < depth; i++ {
		next := &hn.Comment{ID: 1000 + i, By: "b", Text: "reply with a https://very-long-url.example.com/" + strings.Repeat("x", 120)}
		cur.Children = []*hn.Comment{next}
		cur = next
	}
	return []*hn.Comment{root, {ID: 1, Deleted: true, Children: []*hn.Comment{{ID: 2, By: "c", Text: "orphan"}}}}
}

func TestRenderAllScreens(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {40, 12}, {200, 60}} {
		m := newModel(t, size[0], size[1])
		s := m.s
		checkFrame(t, m) // loading

		m.Update(app.FeedLoaded{Gen: s.Update(app.CmdRefresh)[0].(app.FetchFeed).Gen, IDs: []int{1, 2}})
		items := []hn.Item{
			{ID: 1, Title: strings.Repeat("A very long title ", 20), URL: "https://www.example.com/a", Score: 5, By: "pg", Descendants: 1, Kids: []int{1000}},
			{ID: 2, Title: "日本語のタイトル", Score: 1},
		}
		m.Update(app.ItemsLoaded{Gen: s.ListGenForTest(), Items: items})
		checkFrame(t, m)
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}) // light theme
		checkFrame(t, m)

		m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if s.Screen != app.ScreenDetail {
			t.Fatal("enter didn't open the story")
		}
		checkFrame(t, m) // loading comments
		m.Update(app.ThreadLoaded{Gen: s.DetailGenForTest(), Tree: deepTree(20)})
		for range 25 {
			m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
			checkFrame(t, m)
		}
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
		checkFrame(t, m)
		m.Update(tea.KeyMsg{Type: tea.KeyEsc})

		s.Update(app.Resolved{}) // no-op: nothing resolving
		s.Screen, s.ResolveErr = app.ScreenError, hn.ErrNotFound
		if out := checkFrame(t, m); size[0] >= 80 && !strings.Contains(out, "That item is gone") {
			t.Error("error screen missing its message")
		}
	}
}

func TestTooSmall(t *testing.T) {
	m := newModel(t, 20, 5)
	if !strings.Contains(m.View(), "too small") {
		t.Fatal("expected too-small notice")
	}
}
