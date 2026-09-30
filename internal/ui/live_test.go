//go:build live

// Renders real frames from live data, for eyeballing layout:
//
//	go test -tags live -run TestLiveFrames -v ./internal/ui/
package ui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/cintelis/hackernews/internal/app"
)

// settle runs effects synchronously until the app stops asking for work.
func settle(m *Model, cmds []tea.Cmd) {
	for len(cmds) > 0 {
		c := cmds[0]
		cmds = cmds[1:]
		if c == nil {
			continue
		}
		switch msg := c().(type) {
		case tea.BatchMsg:
			cmds = append(cmds, msg...)
		case spinTick, nil:
		default:
			_, next := m.Update(msg)
			cmds = append(cmds, next)
		}
	}
}

func key(m *Model, k tea.KeyMsg) {
	_, c := m.Update(k)
	settle(m, []tea.Cmd{c})
}

func TestLiveFrames(t *testing.T) {
	m := newModel(t, 100, 30)
	settle(m, m.runAll(m.s.Start()))
	fmt.Println(ansi.Strip(checkFrame(t, m)))

	for i, it := range m.s.List.Items {
		if it.Descendants > 5 {
			m.s.List.Cursor = i
			break
		}
	}
	key(m, tea.KeyMsg{Type: tea.KeyEnter})
	for range 3 {
		key(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}
	fmt.Println(ansi.Strip(checkFrame(t, m)))

	key(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	fmt.Println(ansi.Strip(checkFrame(t, m)))
	if m.s.Mode() != app.ModeHelp {
		t.Fatal("help didn't open")
	}
}
