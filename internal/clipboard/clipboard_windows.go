//go:build windows

package clipboard

import (
	"errors"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	kernel32         = windows.NewLazySystemDLL("kernel32.dll")
	openClipboard    = user32.NewProc("OpenClipboard")
	closeClipboard   = user32.NewProc("CloseClipboard")
	emptyClipboard   = user32.NewProc("EmptyClipboard")
	setClipboardData = user32.NewProc("SetClipboardData")
	globalAlloc      = kernel32.NewProc("GlobalAlloc")
	globalLock       = kernel32.NewProc("GlobalLock")
	globalUnlock     = kernel32.NewProc("GlobalUnlock")
	globalFree       = kernel32.NewProc("GlobalFree")
	moveMemory       = kernel32.NewProc("RtlMoveMemory")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

// copyText uses the Windows clipboard directly, so it works in every
// terminal, Windows Terminal or not.
func copyText(text string) error {
	u, err := windows.UTF16FromString(text) // NUL-terminated, as CF_UNICODETEXT wants
	if err != nil {
		return err
	}
	// the clipboard is opened per thread: keep this goroutine on one
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// another program may hold the clipboard for a moment
	var opened uintptr
	for i := 0; i < 10 && opened == 0; i++ {
		if opened, _, _ = openClipboard.Call(0); opened == 0 {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if opened == 0 {
		return errors.New("the clipboard is busy")
	}
	defer closeClipboard.Call()

	if r, _, err := emptyClipboard.Call(); r == 0 {
		return err
	}
	size := uintptr(len(u)) * 2
	h, _, err := globalAlloc.Call(gmemMoveable, size)
	if h == 0 {
		return err
	}
	p, _, err := globalLock.Call(h)
	if p == 0 {
		globalFree.Call(h)
		return err
	}
	moveMemory.Call(p, uintptr(unsafe.Pointer(&u[0])), size)
	globalUnlock.Call(h)
	if r, _, err := setClipboardData.Call(cfUnicodeText, h); r == 0 {
		globalFree.Call(h)
		return err
	}
	return nil // the clipboard owns h now
}
