# Screenshots for the README, recorded with VHS (github.com/charmbracelet/vhs)
# against live Hacker News data. From the repo root:
#
#   GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=0.5.0" -o cintelis-linux ./cmd/cintelis
#   docker run --rm -v "$PWD:/vhs" ghcr.io/charmbracelet/vhs docs/screenshots.tape
#   rm cintelis-linux docs/screenshots/recording.gif

Output docs/screenshots/recording.gif # a by-product; only the PNGs are kept
Set Shell "bash"
Set FontFamily "JetBrains Mono"
Set FontSize 15
Set Width 1280
Set Height 760
Set Padding 0
Set Margin 0
Set WindowBar ""
Set Theme { "name": "CISO AI", "background": "#080b0f", "foreground": "#d7e3ef", "cursor": "#00c8ff", "selection": "#182330", "black": "#080b0f", "red": "#ff6b35", "green": "#00ff88", "yellow": "#e8b04b", "blue": "#60a5fa", "magenta": "#b38ad6", "cyan": "#00c8ff", "white": "#d7e3ef", "brightBlack": "#60768d", "brightRed": "#ff6b35", "brightGreen": "#00ff88", "brightYellow": "#e8b04b", "brightBlue": "#60a5fa", "brightMagenta": "#b38ad6", "brightCyan": "#00c8ff", "brightWhite": "#f1f6fb" }
Env CINTELIS_NO_UPDATE_CHECK "1"
Env CINTELIS_CONFIG_DIR "/tmp/cintelis"
Env COLORTERM "truecolor"

Hide
Type "clear && ./cintelis-linux" Enter
Sleep 8s
Show

# the story list
Sleep 1s
Screenshot docs/screenshots/stories.png

# the light theme (before opening anything, so no story shows as read)
Type "t"
Sleep 1s
Screenshot docs/screenshots/light.png
Type "t"

# a thread: the story, then its comments
Enter
Sleep 7s
Screenshot docs/screenshots/thread.png

# search, by popularity for the past year
Type "/"
Type "rust"
Ctrl+O
Sleep 5s
Screenshot docs/screenshots/search.png

Type "q"
