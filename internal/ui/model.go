// Package ui is the Bubble Tea shell around app.State: it turns key presses
// into commands via the keymap, runs the effects Update returns, and renders
// the state. It holds no behavior of its own beyond scrolling.
package ui

import (
	"context"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cintelis/hackernews/internal/app"
	"github.com/cintelis/hackernews/internal/browser"
	"github.com/cintelis/hackernews/internal/hn"
	"github.com/cintelis/hackernews/internal/keymap"
	"github.com/cintelis/hackernews/internal/store"
	"github.com/cintelis/hackernews/internal/update"
)

const (
	headerHeight = 3
	statusHeight = 2
	fetchTimeout = 45 * time.Second
)

type spinTick struct{}

type Model struct {
	s              *app.State
	client         *hn.Client
	saved, history store.File
	version        string
	keys           keymap.Resolver

	width, height int
	listTop       int // first visible story row
	detailTop     int // first visible line of the detail view
	spin          int
	spinning      bool

	wrapWidth int
	wraps     map[int][]string // comment id (story title/text: negative keys) → wrapped lines at wrapWidth
}

func New(s *app.State, client *hn.Client, saved, history store.File, version string) *Model {
	return &Model{s: s, client: client, saved: saved, history: history, version: version, wraps: map[int][]string{}}
}

func (m *Model) Init() tea.Cmd {
	cmds := m.runAll(m.s.Start())
	cmds = append(cmds, m.checkUpdate(), m.startSpinner())
	return tea.Batch(cmds...)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		body := m.bodyHeight()
		return m, m.dispatch(app.Resized{ListPage: body / 2, DetailPage: body / 4})
	case tea.KeyMsg:
		if c := m.keys.Resolve(m.s.Mode(), msg.String(), time.Now()); c != app.CmdNone {
			return m, m.dispatch(c)
		}
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && m.s.Mode() != app.ModeHelp {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				return m, m.dispatch(app.CmdUp)
			case tea.MouseButtonWheelDown:
				return m, m.dispatch(app.CmdDown)
			}
		}
	case spinTick:
		if m.s.Busy() {
			m.spin++
			return m, tickSpinner()
		}
		m.spinning = false
	case app.FeedLoaded, app.ItemsLoaded, app.ThreadLoaded, app.Resolved, app.UpdateFound, app.Flash:
		return m, m.dispatch(msg)
	}
	return m, nil
}

func (m *Model) dispatch(a app.Action) tea.Cmd {
	cmds := m.runAll(m.s.Update(a))
	cmds = append(cmds, m.startSpinner())
	return tea.Batch(cmds...)
}

func (m *Model) startSpinner() tea.Cmd {
	if m.spinning || !m.s.Busy() {
		return nil
	}
	m.spinning = true
	return tickSpinner()
}

func tickSpinner() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return spinTick{} })
}

func (m *Model) runAll(effs []app.Effect) []tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(effs))
	for _, e := range effs {
		if c := m.run(e); c != nil {
			cmds = append(cmds, c)
		}
	}
	return cmds
}

// run performs one effect. Network work runs as a tea.Cmd off the UI
// goroutine; saves run inline, so they happen in the order they were asked for.
func (m *Model) run(e app.Effect) tea.Cmd {
	client := m.client
	switch e := e.(type) {
	case app.FetchFeed:
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
			defer cancel()
			if e.Purge {
				client.Purge()
			}
			ids, err := client.Feed(ctx, e.Feed)
			return app.FeedLoaded{Gen: e.Gen, IDs: ids, Err: err}
		}
	case app.FetchItems:
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
			defer cancel()
			if e.Purge {
				client.Invalidate(e.IDs...)
			}
			return app.ItemsLoaded{Gen: e.Gen, Items: client.Items(ctx, e.IDs)}
		}
	case app.FetchThread:
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
			defer cancel()
			story := e.Story
			var refreshed *hn.Item
			if e.Refresh {
				client.Invalidate(story.ID)
				if it, err := client.Item(ctx, story.ID); err == nil {
					story, refreshed = it, &it
				}
			}
			tree, err := client.Thread(ctx, story)
			return app.ThreadLoaded{Gen: e.Gen, Story: refreshed, Tree: tree, Err: err}
		}
	case app.ResolveLink:
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
			defer cancel()
			story, focus, err := client.Resolve(ctx, e.Ref)
			return app.Resolved{Gen: e.Gen, Story: story, Focus: focus, Err: err}
		}
	case app.OpenURL:
		return func() tea.Msg {
			if err := browser.Open(e.URL); err != nil {
				return app.Flash{Text: "couldn't open link: " + err.Error()}
			}
			return nil
		}
	case app.SaveSaved:
		return flashOnError(m.saved.Save(e.Entries), "couldn't save saved posts")
	case app.SaveHistory:
		return flashOnError(m.history.Save(e.Entries), "couldn't save history")
	case app.Quit:
		return tea.Quit
	}
	return nil
}

func flashOnError(err error, what string) tea.Cmd {
	if err == nil {
		return nil
	}
	return func() tea.Msg { return app.Flash{Text: what + ": " + err.Error()} }
}

// checkUpdate looks for a newer release once per launch. Release builds only;
// CINTELIS_NO_UPDATE_CHECK=1 turns it off.
func (m *Model) checkUpdate() tea.Cmd {
	if m.version == "dev" || os.Getenv("CINTELIS_NO_UPDATE_CHECK") != "" {
		return nil
	}
	current := m.version
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		v, err := update.Latest(ctx)
		if err != nil || update.Compare(v, current) <= 0 {
			return nil
		}
		return app.UpdateFound{Version: v}
	}
}

func (m *Model) bodyHeight() int { return max(1, m.height-headerHeight-statusHeight) }
