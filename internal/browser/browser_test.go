package browser

import (
	"errors"
	"reflect"
	"testing"
)

func TestCommand(t *testing.T) {
	const u = "https://example.com/?a=1&calc|x"
	got, err := Command("windows", u)
	// the URL must reach rundll32 as one argument: no cmd.exe to split it on & or |
	if err != nil || !reflect.DeepEqual(got, []string{"rundll32", "url.dll,FileProtocolHandler", u}) {
		t.Fatalf("windows: %v, %v", got, err)
	}
	if got, _ := Command("darwin", u); got[0] != "open" || got[1] != u {
		t.Errorf("darwin: %v", got)
	}
	if got, _ := Command("linux", u); got[0] != "xdg-open" || got[1] != u {
		t.Errorf("linux: %v", got)
	}
}

func TestCommandRejectsUnsafe(t *testing.T) {
	for _, u := range []string{
		"", "javascript:alert(1)", "file:///etc/passwd", "ms-settings:", "-a Calculator",
		"//example.com", "/relative", "https://", "ftp://example.com",
	} {
		if _, err := Command("windows", u); !errors.Is(err, ErrUnsafeURL) {
			t.Errorf("%q: err = %v, want ErrUnsafeURL", u, err)
		}
	}
}
