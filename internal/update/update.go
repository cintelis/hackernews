// Package update checks GitHub for newer releases and replaces the running
// binary, verifying the download against the release's checksums.txt.
package update

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Repo is where releases are published.
const Repo = "cintelis/hackernews"

const maxDownload = 200 << 20

// ReleaseKey signs every release's checksums.txt. Its private half is kept
// offline by the maintainer, never on GitHub (see scripts/sign-release.sh).
// The same key appears in install.sh and install.ps1; a test keeps them equal.
//
//go:embed release_key.pub
var ReleaseKey string

// SigNamespace scopes the signatures: this key signs nothing else.
const SigNamespace = "cintelis-release"

var tagRe = regexp.MustCompile(`/tag/v(\d+\.\d+\.\d+)$`)

// Latest returns the newest release version ("1.2.3"). It reads the redirect
// of releases/latest: no API token, no rate limit.
func Latest(ctx context.Context) (string, error) {
	client := &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, "https://github.com/"+Repo+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	res.Body.Close()
	m := tagRe.FindStringSubmatch(res.Header.Get("Location"))
	if m == nil {
		return "", errors.New("no release found")
	}
	return m[1], nil
}

// Compare orders versions: negative if a < b, 0 if equal, positive if a > b.
// Anything unparsable (a "dev" build) sorts lowest.
func Compare(a, b string) int {
	pa, pb := parse(a), parse(b)
	for i := range pa {
		if pa[i] != pb[i] {
			return pa[i] - pb[i]
		}
	}
	return 0
}

func parse(v string) [3]int {
	var out [3]int
	parts := strings.SplitN(strings.TrimPrefix(v, "v"), ".", 3)
	if len(parts) != 3 {
		return [3]int{-1, -1, -1}
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return [3]int{-1, -1, -1}
		}
		out[i] = n
	}
	return out
}

// AssetName matches the archive names in .goreleaser.yaml.
func AssetName(version, goos, goarch string) string {
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("cintelis_%s_%s_%s.%s", version, goos, goarch, ext)
}

// Run is `cintelis update`; it returns the process exit code.
func Run(current string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	latest, err := Latest(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cintelis: couldn't check for releases: %v\n", err)
		return 1
	}
	if Compare(latest, current) <= 0 {
		fmt.Printf("cintelis %s is up to date (latest release is v%s).\n", current, latest)
		return 0
	}
	fmt.Printf("cintelis %s → v%s\n", current, latest)

	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "cintelis: can't locate this executable: %v\n", err)
		return 1
	}
	if HomebrewManaged(exe) {
		fmt.Println("This copy was installed with Homebrew — update it with:\n  brew upgrade cintelis")
		return 0
	}
	if err := install(ctx, latest, exe); err != nil {
		fmt.Fprintf(os.Stderr, "cintelis: update failed: %v\n", err)
		return 1
	}
	fmt.Printf("updated to v%s (%s)\n", latest, exe)
	return 0
}

func install(ctx context.Context, version, exe string) error {
	asset := AssetName(version, runtime.GOOS, runtime.GOARCH)
	base := fmt.Sprintf("https://github.com/%s/releases/download/v%s/", Repo, version)

	sums, err := download(ctx, base+"checksums.txt")
	if err != nil {
		return fmt.Errorf("checksums: %w", err)
	}
	sig, err := download(ctx, base+"checksums.txt.sig")
	if err != nil {
		return fmt.Errorf("release signature: %w", err)
	}
	// the signature is what makes the checksums trustworthy: they come from
	// the same place as the archives, and a forged release forges both
	if err := VerifySSHSig(ReleaseKey, SigNamespace, sums, sig); err != nil {
		return fmt.Errorf("v%s isn't signed with the Cintelis release key (%v) — not installing it", version, err)
	}
	want, err := ChecksumFor(sums, asset)
	if err != nil {
		return err
	}
	fmt.Printf("downloading %s ...\n", asset)
	archive, err := download(ctx, base+asset)
	if err != nil {
		return err
	}
	if got := sha256.Sum256(archive); hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("checksum mismatch for %s — not installing it", asset)
	}
	bin, err := Extract(archive, runtime.GOOS)
	if err != nil {
		return err
	}
	return replace(exe, bin)
}

func download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d for %s", res.StatusCode, url)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxDownload+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxDownload {
		return nil, errors.New("download too large")
	}
	return data, nil
}

// ChecksumFor finds an asset's sha256 in a checksums.txt ("<hex>  <name>").
func ChecksumFor(sums []byte, asset string) (string, error) {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == asset && len(f[0]) == 64 {
			return strings.ToLower(f[0]), nil
		}
	}
	return "", fmt.Errorf("%s isn't listed in checksums.txt", asset)
}

// Extract pulls the cintelis binary out of a release archive. Only that one
// entry is read, by base name, so archive paths never reach the filesystem.
func Extract(archive []byte, goos string) ([]byte, error) {
	if goos == "windows" {
		zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, err
		}
		for _, f := range zr.File {
			if filepath.Base(f.Name) == "cintelis.exe" {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return io.ReadAll(io.LimitReader(rc, maxDownload))
			}
		}
		return nil, errors.New("cintelis.exe not found in archive")
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, errors.New("cintelis not found in archive")
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag == tar.TypeReg && filepath.Base(h.Name) == "cintelis" {
			return io.ReadAll(io.LimitReader(tr, maxDownload))
		}
	}
}

// replace swaps the executable. The new file is written beside it so the
// final rename stays on one filesystem. Windows can't overwrite a running
// .exe but can rename it, so the old one steps aside to exe+".old", which
// CleanupOld removes on a later start.
func replace(exe string, bin []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(exe), ".cintelis-update-*")
	if err != nil {
		return fmt.Errorf("%w (is %s writable?)", err, filepath.Dir(exe))
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(bin); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		old := exe + ".old"
		os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			return err
		}
		if err := os.Rename(tmp.Name(), exe); err != nil {
			os.Rename(old, exe) // put the working binary back
			return err
		}
		return nil
	}
	return os.Rename(tmp.Name(), exe)
}

// CleanupOld removes the binary a Windows update left behind.
func CleanupOld() {
	if runtime.GOOS != "windows" {
		return
	}
	if exe, err := os.Executable(); err == nil {
		os.Remove(exe + ".old")
	}
}

// HomebrewManaged reports whether exe lives in a Homebrew Cellar. Homebrew
// installs are upgraded with brew, which also keeps its records straight.
func HomebrewManaged(exe string) bool {
	return strings.Contains(filepath.ToSlash(exe), "/Cellar/cintelis/")
}

// UpgradeCommand is what to run to update this copy.
func UpgradeCommand() string {
	if exe, err := os.Executable(); err == nil {
		if real, err := filepath.EvalSymlinks(exe); err == nil && HomebrewManaged(real) {
			return "brew upgrade cintelis"
		}
	}
	return "cintelis update"
}
