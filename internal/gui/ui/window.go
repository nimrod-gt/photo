package ui

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"github.com/go-gl/glfw/v3.4/glfw"
)

const (
	defaultWindowWidth  = 1200
	defaultWindowHeight = 800
	leftPanelWidth      = 250
)

type MainWindow struct {
	window      fyne.Window
	actionPanel *ActionPanel
	fileBrowser *FileBrowser
	viewer      *Viewer
	gridViewer  *GridViewer
	notifier    *Notifier
	viewStack   *fyne.Container
	split       *container.Split
}

func NewMainWindow(app fyne.App, actionPanel *ActionPanel, fileBrowser *FileBrowser, viewer *Viewer, gridViewer *GridViewer, notifier *Notifier) *MainWindow {
	w := app.NewWindow("Photo Viewer")
	w.Resize(fyne.NewSize(defaultWindowWidth, defaultWindowHeight))

	mw := &MainWindow{
		window:      w,
		actionPanel: actionPanel,
		fileBrowser: fileBrowser,
		viewer:      viewer,
		gridViewer:  gridViewer,
		notifier:    notifier,
	}
	mw.build()
	return mw
}

func (mw *MainWindow) Window() fyne.Window {
	return mw.window
}

func (mw *MainWindow) ShowError(msg string) {
	mw.notifier.ShowError(msg)
}

func (mw *MainWindow) ShowWarning(msg string) {
	mw.notifier.ShowWarning(msg)
}

func (mw *MainWindow) ShowNotification(msg string) {
	mw.notifier.ShowNotification(msg)
}

// The split offset is a ratio, so a maximized window would hand the left panel
// its share of the whole width instead of the 250px it is meant to keep.
func (mw *MainWindow) Maximize() {
	if area, ok := workArea(mw.window.Canvas().Scale()); ok {
		mw.split.SetOffset(panelOffset(area.Width))
	}
	maximizeWindow(mw.window)
}

func panelOffset(width float32) float64 {
	if width < leftPanelWidth {
		return 1
	}
	return float64(leftPanelWidth) / float64(width)
}

// glfw reports the area in screen coordinates, which are pixels wherever the
// desktop is scaled, while a window is resized in Fyne coordinates.
func workArea(scale float32) (area fyne.Size, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("workArea: recovered from panic: %v", r)
			ok = false
		}
	}()
	monitor := glfw.GetPrimaryMonitor()
	if monitor == nil || scale <= 0 {
		return fyne.Size{}, false
	}
	_, _, width, height := monitor.GetWorkarea()
	if width <= 0 || height <= 0 {
		return fyne.Size{}, false
	}
	return fyne.NewSize(float32(width)/scale, float32(height)/scale), true
}

func (mw *MainWindow) Show() {
	mw.window.ShowAndRun()
}

func (mw *MainWindow) SetGridMode(grid bool) {
	mw.viewStack.RemoveAll()
	if grid {
		mw.viewStack.Add(mw.gridViewer.Container())
		mw.actionPanel.Container().Hide()
	} else {
		mw.viewStack.Add(mw.viewer.Container())
		mw.actionPanel.Container().Show()
	}
	mw.viewStack.Refresh()
}

func (mw *MainWindow) build() {
	mw.viewStack = container.NewStack(mw.viewer.Container())
	rightPanel := container.NewBorder(mw.actionPanel.Container(), nil, nil, nil, mw.viewStack)
	rightWithNotifier := container.NewStack(rightPanel, mw.notifier.Container())

	mw.split = container.NewHSplit(mw.fileBrowser.Container(), rightWithNotifier)
	mw.split.SetOffset(float64(leftPanelWidth) / float64(defaultWindowWidth))

	mw.window.SetContent(mw.split)
}
