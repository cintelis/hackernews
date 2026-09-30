package clipboard

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func TestOSC52Sequence(t *testing.T) {
	t.Setenv("TMUX", "")
	var b bytes.Buffer
	if err := osc52Copy(&b, "héllo"); err != nil {
		t.Fatal(err)
	}
	want := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte("héllo")) + "\a"
	if b.String() != want {
		t.Fatalf("sequence = %q, want %q", b.String(), want)
	}
	b.Reset()
	t.Setenv("TMUX", "/tmp/tmux-1/default,1,0")
	osc52Copy(&b, "x")
	if !strings.HasPrefix(b.String(), "\x1bPtmux;") {
		t.Fatalf("inside tmux the sequence should be wrapped: %q", b.String())
	}
}
