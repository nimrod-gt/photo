//go:build darwin

package nativewin

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdint.h>

void maximizeNSWindow(uintptr_t handle);
*/
import "C"

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

// Resizing through Fyne sets the size of the content, so the title bar on top of
// it pushes the window past the visible area and the top of the content ends up
// underneath. AppKit sizes the whole frame instead.
func Maximize(window fyne.Window) {
	native, ok := window.(driver.NativeWindow)
	if !ok {
		return
	}
	native.RunNative(func(context any) {
		mac, ok := context.(driver.MacWindowContext)
		if !ok || mac.NSWindow == 0 {
			return
		}
		C.maximizeNSWindow(C.uintptr_t(mac.NSWindow))
	})
}
