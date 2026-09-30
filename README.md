# cintelis

A fast Hacker News reader for the terminal. One small binary for macOS,
Linux and Windows, no runtime, no account, no API keys.

## Install

**macOS / Linux**

```sh
curl -fsSL https://raw.githubusercontent.com/cintelis/hackernews/main/install.sh | sh
```

**Windows** (PowerShell)

```powershell
irm https://raw.githubusercontent.com/cintelis/hackernews/main/install.ps1 | iex
```

Both scripts install the latest release only if its checksums are signed
with the cintelis release key and the download matches them. Or, with Go installed:

```sh
go install github.com/cintelis/hackernews/cmd/cintelis@latest
```

Binaries are also on the [releases page](https://github.com/cintelis/hackernews/releases).
See [SECURITY.md](SECURITY.md) to verify one by hand, or to report a
vulnerability.

## Use

```
cintelis            browse
cintelis update     update to the latest release (checksum-verified)
cintelis version    print the version
```

Press `?` for every key. The essentials:

| Key | List | Thread |
| --- | --- | --- |
| `j` `k` / arrows | move | move |
| `gg` `G`, `ctrl+d` `ctrl+u` | top / bottom, half page | same |
| `h` `l` / `tab`, `1`–`6` | switch tab | `h` goes back |
| `⏎` | open comments | links in the comment |
| `space` | | collapse / expand |
| `n` | | newest comments first / ranked |
| `o` / `y` | open link / HN page in browser | same |
| `s`, `S`, `H` | save, saved posts, history | same |
| `r` | refresh | refresh |
| `/` | search all of Hacker News | same |
| `t`, `q` | theme, quit | same |

**Search** (`/`) looks through every story title on Hacker News as you type,
and tolerates typos ("kubernets" finds Kubernetes). If no title has all your
words, it shows the closest matches instead, and if Hacker News search can't
be reached it fuzzy-matches the stories you've already loaded. `⏎` or `↓`
moves into the results, `esc` stops typing.

In a thread, `n` switches to **newest first**: every comment in one list,
latest on top, each marked with who it replies to. The cursor lands on the
newest comment, and the switch stays on for the threads you open next; `n`
again returns to HN's ranked order on the same comment. Points are Hacker
News's own score for a story (its upvotes); HN doesn't publish comment scores.

Links to other HN posts open inside the app, landing on the linked comment;
`h`/`esc` walks back through them.

## Data and privacy

- Stories come from the official [HN API](https://github.com/HackerNews/API)
  (`hacker-news.firebaseio.com`) and whole comment threads from the
  [Algolia HN API](https://hn.algolia.com/api) (`hn.algolia.com`). Both are
  public and read-only.
- At startup it asks `github.com` whether a newer release exists. Set
  `CINTELIS_NO_UPDATE_CHECK=1` to skip that.
- Saved posts and history stay on your machine in `~/.config/cintelis`
  (`saved.json`, `history.json`; override with `CINTELIS_CONFIG_DIR`).
- No telemetry.

## Development

```sh
go test ./...                            # unit tests, no network
go test -tags live ./internal/hn/        # against the real APIs
go test -tags live -run TestLiveFrames -v ./internal/ui/   # print real frames
go run ./cmd/cintelis
```

```
cmd/cintelis/     entry point, subcommands
internal/app/     State + Update(action) → effects: all behavior, no I/O
internal/keymap/  every binding as data; help and hints are generated from it
internal/ui/      Bubble Tea shell: keys → commands, runs effects, renders
internal/hn/      API client, item cache, HTML → text, link parsing
internal/store/   saved/history files, atomic writes
internal/browser/ opens http(s) links without a shell
internal/update/  release check and checksum-verified self-update
```

`app` is the part to read first: every rule of navigation lives in
`update.go` and is covered by `update_test.go` without a terminal.

## Releasing

Tag and push; the release workflow tests, builds all six targets with
GoReleaser and attests them, as a **draft**. Then sign it with the offline
release key, which checks the draft's provenance and checksums first and
publishes it:

```sh
git tag v1.0.0
git push origin v1.0.0
# once the workflow finishes:
scripts/sign-release.sh v1.0.0
```

Nothing is installable until it's signed.

## License

MIT, see [LICENSE](LICENSE).
