package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"photo/internal/gui/nativewin"
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
	tagBar      *TagBar
	runBar      *RunBar
	viewStack   *fyne.Container
}

func NewMainWindow(app fyne.App, actionPanel *ActionPanel, fileBrowser *FileBrowser, viewer *Viewer, gridViewer *GridViewer, notifier *Notifier, tagBar *TagBar, runBar *RunBar) *MainWindow {
	w := app.NewWindow("Photo Viewer")
	w.Resize(fyne.NewSize(defaultWindowWidth, defaultWindowHeight))

	mw := &MainWindow{
		window:      w,
		actionPanel: actionPanel,
		fileBrowser: fileBrowser,
		viewer:      viewer,
		gridViewer:  gridViewer,
		notifier:    notifier,
		tagBar:      tagBar,
		runBar:      runBar,
	}
	mw.build()
	return mw
}

func (mw *MainWindow) Window() fyne.Window {
	return mw.window
}

func (mw *MainWindow) Maximize() {
	nativewin.Maximize(mw.window)
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
	mw.tagBar.SetSuppressed(grid)
	mw.viewStack.Refresh()
}

func (mw *MainWindow) build() {
	mw.viewStack = container.NewStack(mw.viewer.Container())
	withTags := container.NewStack(mw.viewStack, mw.tagBar.Container(), mw.runBar.Container())
	rightPanel := container.NewBorder(mw.actionPanel.Container(), nil, nil, nil, withTags)
	rightWithNotifier := container.NewStack(rightPanel, mw.notifier.Container())

	split := container.NewHSplit(mw.fileBrowser.Container(), rightWithNotifier)
	split.SetOffset(float64(leftPanelWidth) / float64(defaultWindowWidth))

	mw.window.SetContent(container.New(newPanelWidthKeeper(split), split))
}
