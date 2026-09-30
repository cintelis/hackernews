package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

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

// With a colour terminal, the brand palette must reach the output: the CA
// mark on brand cyan, the dark body background.
func TestBrandColors(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)
	out := newModel(t, 80, 24).View()
	for name, seq := range map[string]string{
		"CA mark on #00c8ff": "48;2;0;200;255",
		"body #080b0f":       "48;2;8;11;15",
	} {
		if !strings.Contains(out, seq) {
			t.Errorf("%s missing from the frame", name)
		}
	}
	if !strings.Contains(ansi.Strip(out), "CA  CISO AI") {
		t.Errorf("brand text missing: %q", strings.SplitN(ansi.Strip(out), "\n", 2)[0])
	}
}

func TestRenderSearch(t *testing.T) {
	m := newModel(t, 80, 24)
	s := m.s
	s.Start()
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if out := ansi.Strip(checkFrame(t, m)); !strings.Contains(out, "Search every story") {
		t.Fatal("empty search screen missing its prompt")
	}
	for _, r := range "kubernets" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	if s.Search.Query != "kubernets " {
		t.Fatalf("typed query = %q", s.Search.Query)
	}
	checkFrame(t, m) // searching…
	m.Update(app.SearchLoaded{Gen: s.ListGenForTest(), Query: "kubernets", Kind: hn.SearchAnyWord,
		Items: []hn.Item{{ID: 1, Title: "Kubernetes in production", Score: 3}}})
	out := ansi.Strip(checkFrame(t, m))
	if !strings.Contains(out, "/ kubernets") || !strings.Contains(out, "closest matches") || !strings.Contains(out, "Kubernetes in production") {
		t.Fatalf("search frame:\n%s", out)
	}
}

func TestRenderNewestFirst(t *testing.T) {
	m := newModel(t, 80, 24)
	s := m.s
	m.Update(app.FeedLoaded{Gen: s.Update(app.CmdRefresh)[0].(app.FetchFeed).Gen, IDs: []int{1}})
	m.Update(app.ItemsLoaded{Gen: s.ListGenForTest(), Items: []hn.Item{{ID: 1, Title: "t", Descendants: 2}}})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.Update(app.ThreadLoaded{Gen: s.DetailGenForTest(), Tree: []*hn.Comment{
		{ID: 10, By: "pg", Time: 1, Text: "first", Children: []*hn.Comment{{ID: 11, By: "dang", Time: 2, Text: "reply"}}},
	}})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	out := ansi.Strip(checkFrame(t, m))
	for _, want := range []string{"newest first (n)", "dang", "↳ pg", "newest comments first"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderSearchFilters(t *testing.T) {
	m := newModel(t, 100, 24)
	m.s.Start()
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	out := ansi.Strip(checkFrame(t, m))
	if !strings.Contains(out, "Search Stories by Date for Past year") || !strings.Contains(out, "ctrl+r range") {
		t.Fatalf("filter line missing:\n%s", out)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m.Update(app.SearchLoaded{Gen: m.s.ListGenForTest(), Items: []hn.Item{{ID: 1, Title: "Show HN: x"}}})
	out = ansi.Strip(checkFrame(t, m))
	if !strings.Contains(out, "Search Show HN by Date") || !strings.Contains(out, "1 result") || !strings.Contains(out, "Show HN: x") {
		t.Fatalf("browse frame:\n%s", out)
	}
}
