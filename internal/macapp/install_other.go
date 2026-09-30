//go:build !darwin

package macapp

import "errors"

// ErrNotMac: the app bundle is macOS-only. On Windows the installer adds a
// Start menu entry instead.
var ErrNotMac = errors.New("the app bundle is for macOS (on Windows, the installer adds a Start menu entry)")

func Install(string) (string, error) { return "", ErrNotMac }
