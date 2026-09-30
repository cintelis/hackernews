package store

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	f := Saved(t.TempDir())
	want := []Entry{{ID: 2, At: 200}, {ID: 1, At: 100}}
	if err := f.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := f.Load()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Load = %v, %v; want %v", got, err, want)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(f.Path), ".tmp-*")); len(leftovers) > 0 {
		t.Errorf("temp files left behind: %v", leftovers)
	}
}

// Hand-edited or damaged entries are skipped, not fatal.
func TestLoadSkipsBadEntries(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "history.json"), []byte(`[
  {"id": 41, "viewedAt": 1727000000000},
  {"id": "bad", "viewedAt": 1},
  {"viewedAt": 5},
  {"id": 42, "viewedAt": 1727000001000}
]`), 0o644)
	got, err := History(dir).Load()
	want := []Entry{{ID: 41, At: 1727000000000}, {ID: 42, At: 1727000001000}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Load = %v, %v; want %v", got, err, want)
	}
}

func TestSaveFormat(t *testing.T) {
	f := Saved(t.TempDir())
	f.Save([]Entry{{ID: 5, At: 9}})
	data, _ := os.ReadFile(f.Path)
	if !strings.Contains(string(data), `"savedAt": 9`) || !strings.Contains(string(data), `"id": 5`) {
		t.Fatalf("unexpected file contents: %s", data)
	}
}

func TestMissingAndCorrupt(t *testing.T) {
	dir := t.TempDir()
	if got, err := Saved(dir).Load(); err != nil || got != nil {
		t.Fatalf("missing file: %v, %v", got, err)
	}
	os.WriteFile(filepath.Join(dir, "saved.json"), []byte("{not json"), 0o644)
	if _, err := Saved(dir).Load(); err == nil {
		t.Fatal("corrupt file should be an error, not silently empty")
	}
}

func TestSaveCreatesDir(t *testing.T) {
	f := Saved(filepath.Join(t.TempDir(), "a", "b"))
	if err := f.Save(nil); err != nil {
		t.Fatal(err)
	}
	if got, err := f.Load(); err != nil || len(got) != 0 {
		t.Fatalf("Load = %v, %v", got, err)
	}
}
