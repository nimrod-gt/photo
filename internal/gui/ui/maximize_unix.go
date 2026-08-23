//go:build !windows && !darwin

package ui

import (
	"log"

	"fyne.io/fyne/v2"
)

// Windows and macOS maximize through their native window handle. Elsewhere the
// window is grown to the largest size that still fits the work area, which is
// all Fyne can ask for: it sizes the content rather than the frame and can only
// place a window by centring it on the monitor.
func maximizeWindow(window fyne.Window) {
	metrics, ok := screenLayout(window.Canvas().Scale())
	if !ok {
		return
	}
	size, ok := maximizedContentSize(metrics)
	if !ok {
		return
	}
	window.Resize(size)
	window.CenterOnScreen()
}

// glfw reports a monitor in screen coordinates, which are pixels wherever the
// desktop is scaled, while a window is sized in Fyne coordinates.
func screenLayout(scale float32) (metrics screenMetrics, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("screenLayout: recovered from panic: %v", r)
			ok = false
		}
	}()
	mode, monitor := primaryVideoMode()
	if mode == nil || scale <= 0 {
		return screenMetrics{}, false
	}
	_, top, width, height := monitor.GetWorkarea()
	if width <= 0 || height <= 0 {
		return screenMetrics{}, false
	}
	return screenMetrics{
		monitorHeight: float32(mode.Height) / scale,
		areaTop:       float32(top) / scale,
		area:          fyne.NewSize(float32(width)/scale, float32(height)/scale),
	}, true
}
