package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
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
}

func NewMainWindow(app fyne.App, actionPanel *ActionPanel, fileBrowser *FileBrowser, viewer *Viewer) *MainWindow {
	w := app.NewWindow("Photo Viewer")
	w.Resize(fyne.NewSize(defaultWindowWidth, defaultWindowHeight))

	mw := &MainWindow{
		window:      w,
		actionPanel: actionPanel,
		fileBrowser: fileBrowser,
		viewer:      viewer,
	}
	mw.build()
	return mw
}

func (mw *MainWindow) Window() fyne.Window {
	return mw.window
}

func (mw *MainWindow) Show() {
	mw.window.ShowAndRun()
}

func (mw *MainWindow) build() {
	rightPanel := container.NewBorder(mw.actionPanel.Container(), nil, nil, nil, mw.viewer.Container())

	split := container.NewHSplit(mw.fileBrowser.Container(), rightPanel)
	split.SetOffset(float64(leftPanelWidth) / float64(defaultWindowWidth))

	mw.window.SetContent(split)
}
