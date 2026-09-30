// Package store persists saved posts and view history as small JSON files
// in ~/.config/cintelis.
package store

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// HistoryCap bounds the history file.
const HistoryCap = 1000

// Entry is one saved or viewed item and when that happened (unix millis).
type Entry struct {
	ID int
	At int64
}

// File is one JSON list: [{"id": 1, "<timeKey>": 1700000000000}, ...].
type File struct {
	Path    string
	timeKey string
}

func Saved(dir string) File { return File{Path: filepath.Join(dir, "saved.json"), timeKey: "savedAt"} }
func History(dir string) File {
	return File{Path: filepath.Join(dir, "history.json"), timeKey: "viewedAt"}
}

// Dir is where the files live: $CINTELIS_CONFIG_DIR, else ~/.config/cintelis.
func Dir() (string, error) {
	if d := os.Getenv("CINTELIS_CONFIG_DIR"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "cintelis"), nil
}

// Load reads the list. A missing file is an empty list; malformed entries
// are skipped rather than failing the whole file.
func (f File) Load() ([]Entry, error) {
	data, err := os.ReadFile(f.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var raw []map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(raw))
	for _, r := range raw {
		id, ok1 := r["id"].(float64)
		at, ok2 := r[f.timeKey].(float64)
		if ok1 && ok2 && id > 0 {
			out = append(out, Entry{ID: int(id), At: int64(at)})
		}
	}
	return out, nil
}

// Save replaces the file atomically: write a temp file beside it, sync,
// rename over. A crash mid-write leaves the old file intact.
func (f File) Save(entries []Entry) error {
	raw := make([]map[string]int64, len(entries))
	for i, e := range entries {
		raw[i] = map[string]int64{"id": int64(e.ID), f.timeKey: e.At}
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(f.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once renamed
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), f.Path)
}
