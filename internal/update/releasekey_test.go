package update

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The release key must be a real Ed25519 key, and install.sh and
// install.ps1 must carry exactly the key the app does: a mismatch would make
// installers and updates disagree about which releases are genuine.
func TestReleaseKeyConsistent(t *testing.T) {
	key := strings.Join(strings.Fields(ReleaseKey)[:min(2, len(strings.Fields(ReleaseKey)))], " ")
	if _, err := parseEd25519(key); err != nil {
		t.Fatalf("internal/update/release_key.pub is not an ssh-ed25519 public key (%v) — releases can't be verified", err)
	}
	for file, pattern := range map[string]string{
		"../../install.sh":  `(?m)^RELEASE_KEY="([^"]*)"`,
		"../../install.ps1": `(?m)^\$releaseKey = '([^']*)'`,
	} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		m := regexp.MustCompile(pattern).FindSubmatch(src)
		if m == nil {
			t.Errorf("%s: release key line not found", file)
			continue
		}
		if got := string(m[1]); got != key {
			t.Errorf("%s has release key %q, the app has %q", file, got, key)
		}
	}
}
