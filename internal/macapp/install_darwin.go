//go:build darwin

package macapp

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed AppIcon.icns
var icon []byte

// Install writes the app to ~/Applications (no admin rights needed; Launchpad
// and Spotlight look there) and returns its path. It replaces any earlier
// copy, building the new one beside it first so a half-written app never
// shows up.
func Install(version string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	apps := filepath.Join(home, "Applications")
	app := filepath.Join(apps, Name+".app")
	tmp := filepath.Join(apps, "."+Name+".app.new")
	os.RemoveAll(tmp)
	if err := write(tmp, version, selfPath(), icon); err != nil {
		os.RemoveAll(tmp)
		return "", err
	}
	os.RemoveAll(app)
	if err := os.Rename(tmp, app); err != nil {
		return "", err
	}
	now := time.Now()
	os.Chtimes(app, now, now) // a fresh date makes Finder pick up the icon
	return app, nil
}

// selfPath is this binary's stable path: for Homebrew, the prefix's bin/
// link rather than the versioned Cellar path an upgrade removes.
func selfPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if i := strings.Index(exe, "/Cellar/cintelis/"); i >= 0 {
		return exe[:i] + "/bin/cintelis"
	}
	return exe
}
