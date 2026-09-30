package macapp

import (
	"encoding/xml"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteBundle(t *testing.T) {
	app := filepath.Join(t.TempDir(), Name+".app")
	if err := write(app, "0.5.0", "/Users/o'neil/.local/bin/cintelis", []byte("ICON")); err != nil {
		t.Fatal(err)
	}
	// rewriting over an existing bundle works too
	if err := write(app, "0.5.0", "/Users/o'neil/.local/bin/cintelis", []byte("ICON")); err != nil {
		t.Fatal(err)
	}

	// Info.plist is well-formed XML with the keys macOS needs
	plist, err := os.ReadFile(filepath.Join(app, "Contents", "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	d := xml.NewDecoder(strings.NewReader(string(plist)))
	d.Strict = true
	for {
		if _, err := d.Token(); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("Info.plist isn't well-formed: %v", err)
		}
	}
	for _, want := range []string{
		"<string>CISO AI - Hacker News</string>", "<string>cintelis-launcher</string>",
		"<string>AppIcon</string>", "<string>APPL</string>", "<string>0.5.0</string>",
		"<string>ai.cintelis.cisoai.hackernews</string>",
	} {
		if !strings.Contains(string(plist), want) {
			t.Errorf("Info.plist lacks %s", want)
		}
	}

	if b, _ := os.ReadFile(filepath.Join(app, "Contents", "Resources", "AppIcon.icns")); string(b) != "ICON" {
		t.Error("icon not written")
	}

	for _, script := range []string{"Contents/MacOS/cintelis-launcher", "Contents/Resources/cintelis.command"} {
		p := filepath.Join(app, filepath.FromSlash(script))
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
			t.Errorf("%s isn't executable", script)
		}
		if sh, err := exec.LookPath("sh"); err == nil {
			if out, err := exec.Command(sh, "-n", p).CombinedOutput(); err != nil {
				t.Errorf("%s: shell syntax: %v %s", script, err, out)
			}
		}
	}

	// the installing binary's path is tried first, safely quoted
	cmd, _ := os.ReadFile(filepath.Join(app, "Contents", "Resources", "cintelis.command"))
	if !strings.Contains(string(cmd), `for c in '/Users/o'\''neil/.local/bin/cintelis' `) {
		t.Errorf("command file:\n%s", cmd)
	}
}
