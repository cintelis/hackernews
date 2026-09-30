// Package macapp writes "CISO AI - Hacker News.app", a small macOS app bundle
// with the CISO AI icon that opens cintelis in a Terminal window, so it can
// be found in Launchpad and Spotlight and kept in the Dock.
//
// The bundle is written on the user's Mac (`cintelis install-app`), not
// downloaded, so Gatekeeper doesn't quarantine it. It holds no copy of
// cintelis: it runs whichever one is installed, so updates need nothing here.
package macapp

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Name is the app's name in Finder, Launchpad and the Dock.
const Name = "CISO AI - Hacker News"

const bundleID = "ai.cintelis.cisoai.hackernews"

// write builds the bundle at app. cintelis is the binary to try first; the
// launcher also looks on the PATH and in the usual install locations.
func write(app, version, cintelis string, icon []byte) error {
	files := []struct {
		path string
		data []byte
		mode os.FileMode
	}{
		{"Contents/Info.plist", []byte(infoPlist(version)), 0o644},
		{"Contents/MacOS/cintelis-launcher", []byte(launcher), 0o755},
		{"Contents/Resources/cintelis.command", []byte(command(cintelis)), 0o755},
		{"Contents/Resources/AppIcon.icns", icon, 0o644},
	}
	for _, f := range files {
		p := filepath.Join(app, filepath.FromSlash(f.path))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, f.data, f.mode); err != nil {
			return err
		}
		if err := os.Chmod(p, f.mode); err != nil { // WriteFile keeps an existing file's mode
			return err
		}
	}
	return nil
}

func infoPlist(version string) string {
	esc := func(s string) string {
		var b strings.Builder
		xml.EscapeText(&b, []byte(s))
		return b.String()
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>%[1]s</string>
	<key>CFBundleDisplayName</key>
	<string>%[1]s</string>
	<key>CFBundleIdentifier</key>
	<string>%[2]s</string>
	<key>CFBundleExecutable</key>
	<string>cintelis-launcher</string>
	<key>CFBundleIconFile</key>
	<string>AppIcon</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>%[3]s</string>
	<key>CFBundleVersion</key>
	<string>%[3]s</string>
	<key>LSMinimumSystemVersion</key>
	<string>11.0</string>
	<key>LSApplicationCategoryType</key>
	<string>public.app-category.news</string>
	<key>NSHumanReadableCopyright</key>
	<string>Copyright © 2026 Cintelis</string>
</dict>
</plist>
`, esc(Name), bundleID, esc(version))
}

// launcher is the bundle's executable: it asks Terminal to run the
// .command file, which opens a new window with the user's own shell setup.
const launcher = `#!/bin/sh
# Opens cintelis in a new Terminal window.
here=$(cd "$(dirname "$0")/.." && pwd)
exec /usr/bin/open -a Terminal "$here/Resources/cintelis.command"
`

func command(cintelis string) string {
	return fmt.Sprintf(`#!/bin/sh
# Run by Terminal for "%s.app". Finds cintelis wherever it's installed.
for c in %s "$(command -v cintelis 2>/dev/null)" "$HOME/.local/bin/cintelis" /opt/homebrew/bin/cintelis /usr/local/bin/cintelis; do
  if [ -n "$c" ] && [ -x "$c" ]; then
    exec "$c"
  fi
done
echo "cintelis isn't installed any more."
echo "Install it again: https://github.com/cintelis/hackernews#install"
echo "Or remove this app: ~/Applications/%s.app"
`, Name, shellQuote(cintelis), Name)
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
