package ui

import (
	"image"
	"log"

	"fyne.io/fyne/v2"
	"github.com/go-gl/glfw/v3.4/glfw"
)

// Fyne sizes the content of a window, not its frame, and offers no way to read
// how tall the decorations the system draws around it are, so the height is
// given away rather than guessed exactly: a window one title bar short of the
// work area is worth more than one whose title bar sits off the screen.
const decorationAllowance = 48

func MonitorSize() image.Point {
	mode, _ := primaryVideoMode()
	if mode == nil {
		return image.Point{X: 3840, Y: 2160}
	}
	return image.Point{X: mode.Width, Y: mode.Height}
}

// glfw panics rather than failing when there is no window system to ask, so
// the probe is recovered and reports no monitor instead.
func primaryVideoMode() (mode *glfw.VidMode, monitor *glfw.Monitor) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("primaryVideoMode: recovered from panic: %v", r)
			mode, monitor = nil, nil
		}
	}()
	monitor = glfw.GetPrimaryMonitor()
	if monitor == nil {
		return nil, nil
	}
	mode = monitor.GetVideoMode()
	if mode == nil || mode.Width <= 0 || mode.Height <= 0 {
		return nil, nil
	}
	return mode, monitor
}

type screenMetrics struct {
	monitorHeight float32
	areaTop       float32
	area          fyne.Size
}

func maximizedContentSize(metrics screenMetrics) (fyne.Size, bool) {
	height := centrableHeight(metrics) - decorationAllowance
	if metrics.area.Width <= 0 || height <= 0 {
		return fyne.Size{}, false
	}
	return fyne.NewSize(metrics.area.Width, height), true
}

// The window can only be placed by centring it on the whole monitor, so the
// tallest frame it may take is the one that still lands inside the work area
// once centred: a panel along the top costs the same height off the bottom.
func centrableHeight(metrics screenMetrics) float32 {
	fromTop := metrics.monitorHeight - 2*metrics.areaTop
	fromBottom := 2*(metrics.areaTop+metrics.area.Height) - metrics.monitorHeight
	return min(fromTop, fromBottom)
}
