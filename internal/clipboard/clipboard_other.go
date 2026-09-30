//go:build !windows

package clipboard

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// copyText prefers the desktop's own tool; over SSH that would copy on the
// remote machine, so there (and when no tool is installed) it asks the
// terminal instead.
func copyText(text string) error {
	if os.Getenv("SSH_CONNECTION") == "" && os.Getenv("SSH_TTY") == "" {
		for _, argv := range tools() {
			path, err := exec.LookPath(argv[0])
			if err != nil {
				continue
			}
			cmd := exec.Command(path, argv[1:]...)
			cmd.Stdin = strings.NewReader(text)
			if cmd.Run() == nil {
				return nil
			}
		}
	}
	return osc52Copy(os.Stdout, text)
}

func tools() [][]string {
	if runtime.GOOS == "darwin" {
		return [][]string{{"pbcopy"}}
	}
	var t [][]string
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		t = append(t, []string{"wl-copy"})
	}
	if os.Getenv("DISPLAY") != "" {
		t = append(t, []string{"xclip", "-selection", "clipboard"}, []string{"xsel", "--clipboard", "--input"})
	}
	return t
}
