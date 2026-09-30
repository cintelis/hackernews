// cintelis: a Hacker News reader for the terminal.
package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cintelis/hackernews/internal/app"
	"github.com/cintelis/hackernews/internal/hn"
	"github.com/cintelis/hackernews/internal/store"
	"github.com/cintelis/hackernews/internal/ui"
	"github.com/cintelis/hackernews/internal/update"
)

// Windows icon and file details for local builds (release builds do this in
// .goreleaser.yaml, with the real version):
//
//go:generate go run github.com/tc-hib/go-winres@v0.3.3 make --in ../../winres/winres.json --out rsrc --arch amd64,arm64

// version is set at release build time (see .goreleaser.yaml).
var version = "dev"

const usage = `usage: cintelis [command]

  (none)     browse Hacker News
  update     update to the latest release
  version    print the version

Environment:
  CINTELIS_CONFIG_DIR        where saved posts and history live (default ~/.config/cintelis)
  CINTELIS_NO_UPDATE_CHECK   set to skip the release check at startup
`

func main() {
	// `go install …@v1.2.3` builds carry their version in the build info
	if bi, ok := debug.ReadBuildInfo(); ok && version == "dev" && strings.HasPrefix(bi.Main.Version, "v") {
		version = strings.TrimPrefix(bi.Main.Version, "v")
	}
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	update.CleanupOld() // on any run: the binary an update replaced isn't running any more
	if len(args) > 0 {
		switch args[0] {
		case "version", "--version", "-v":
			fmt.Printf("cintelis %s\n", version)
			return 0
		case "update":
			return update.Run(version)
		case "help", "--help", "-h":
			fmt.Print(usage)
			return 0
		default:
			fmt.Fprintf(os.Stderr, "cintelis: unknown command %q\n\n%s", args[0], usage)
			return 1
		}
	}

	dir, err := store.Dir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cintelis: %v\n", err)
		return 1
	}
	savedFile, historyFile := store.Saved(dir), store.History(dir)
	// refuse to start on an unreadable file rather than overwrite it on the next save
	saved, err := savedFile.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cintelis: can't read %s: %v\n", savedFile.Path, err)
		return 1
	}
	history, err := historyFile.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cintelis: can't read %s: %v\n", historyFile.Path, err)
		return 1
	}

	model := ui.New(app.New(saved, history), hn.NewClient(), savedFile, historyFile, version)
	if _, err := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "cintelis: %v\n", err)
		return 1
	}
	return 0
}
