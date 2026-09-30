// Package clipboard puts text on the system clipboard: natively on Windows,
// with pbcopy on macOS, wl-copy/xclip/xsel on Linux desktops, and otherwise
// (over SSH, or with none of those installed) by asking the terminal to do
// it with an OSC 52 escape sequence.
package clipboard

import (
	"io"
	"os"

	"github.com/aymanbagabas/go-osc52/v2"
)

// Copy puts text on the clipboard.
func Copy(text string) error { return copyText(text) }

// osc52Copy asks the terminal to set its clipboard. Windows Terminal, iTerm2,
// kitty, WezTerm, Alacritty and foot honour it; inside tmux it's wrapped so
// tmux passes it through.
func osc52Copy(w io.Writer, text string) error {
	seq := osc52.New(text)
	if os.Getenv("TMUX") != "" {
		seq = seq.Tmux()
	}
	_, err := seq.WriteTo(w)
	return err
}
