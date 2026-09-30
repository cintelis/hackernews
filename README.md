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

Both scripts install the latest release and refuse it if its SHA-256 doesn't
match the release's `checksums.txt`. Or, with Go installed:

```sh
go install github.com/cintelis/hackernews/cmd/cintelis@latest
```

Binaries are also on the [releases page](https://github.com/cintelis/hackernews/releases).
Each release carries a signed build-provenance attestation:
`gh attestation verify <archive> --repo cintelis/hackernews`.

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
| `o` / `y` | open link / HN page in browser | same |
| `s`, `S`, `H` | save, saved posts, history | same |
| `r` | refresh | refresh |
| `t`, `q` | theme, quit | same |

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
GoReleaser, publishes the archives with `checksums.txt`, and attests them.

```sh
git tag v1.0.0
git push origin v1.0.0
```

## License

MIT, see [LICENSE](LICENSE).
