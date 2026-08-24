//go:build windows

package nativewin

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"golang.org/x/sys/windows"
)

var showWindow = windows.NewLazySystemDLL("user32.dll").NewProc("ShowWindow")

func Maximize(window fyne.Window) {
	native, ok := window.(driver.NativeWindow)
	// A LazyProc panics when it is called and cannot be resolved, and this one
	// runs on the goroutine the driver loop lives on.
	if !ok || showWindow.Find() != nil {
		return
	}
	native.RunNative(func(context any) {
		win, ok := context.(driver.WindowsWindowContext)
		if !ok || win.HWND == 0 {
			return
		}
		_, _, _ = showWindow.Call(win.HWND, uintptr(windows.SW_MAXIMIZE))
	})
}
