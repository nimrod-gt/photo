//go:build windows

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"golang.org/x/sys/windows"
)

var showWindow = windows.NewLazySystemDLL("user32.dll").NewProc("ShowWindow")

func maximizeWindow(window fyne.Window) {
	native, ok := window.(driver.NativeWindow)
	if !ok {
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
