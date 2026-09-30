//go:build windows && clipboard

// Touches the real clipboard, so it only runs when asked:
//
//	go test -tags clipboard ./internal/clipboard/
package clipboard

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestCopyRoundTrip(t *testing.T) {
	want := "cintelis clipboard test ✓ 日本語 " + time.Now().Format(time.RFC3339Nano)
	if err := Copy(want); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("powershell.exe", "-NoProfile", "-Command",
		"[Console]::OutputEncoding = [Text.Encoding]::UTF8; Get-Clipboard -Raw").Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimRight(string(out), "\r\n"); got != want {
		t.Fatalf("clipboard = %q, want %q", got, want)
	}
}
