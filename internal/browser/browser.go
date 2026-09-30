// Package browser opens web links in the user's default browser.
package browser

import (
	"errors"
	"net/url"
	"os/exec"
	"runtime"
)

// ErrUnsafeURL: only absolute http(s) URLs are opened. Links come from HN
// content, which anyone can write.
var ErrUnsafeURL = errors.New("not an http(s) link")

// Command builds the launcher invocation for goos, or fails for a URL that
// isn't absolute http(s). No shell is involved on any platform: on Windows,
// `cmd /c start` would run whatever follows an `&` in the URL.
func Command(goos, raw string) ([]string, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, ErrUnsafeURL
	}
	s := u.String()
	switch goos {
	case "windows":
		return []string{"rundll32", "url.dll,FileProtocolHandler", s}, nil
	case "darwin":
		return []string{"open", s}, nil
	default:
		return []string{"xdg-open", s}, nil
	}
}

// Open launches the browser without waiting for it.
func Open(raw string) error {
	argv, err := Command(runtime.GOOS, raw)
	if err != nil {
		return err
	}
	cmd := exec.Command(argv[0], argv[1:]...) // nil stdio → null device, keeps the TUI clean
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
