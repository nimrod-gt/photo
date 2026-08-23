//go:build !windows && !darwin

package ui

import "fyne.io/fyne/v2"

// Windows and macOS maximize through their native window handle; elsewhere the
// window is grown to the area a maximized one would cover.
func maximizeWindow(window fyne.Window) {
	if area, ok := workArea(window.Canvas().Scale()); ok {
		window.Resize(area)
		window.CenterOnScreen()
	}
}
