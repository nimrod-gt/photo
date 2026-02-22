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
	toolbar     *Toolbar
	fileBrowser *FileBrowser
	viewer      *Viewer
}

func NewMainWindow(app fyne.App, toolbar *Toolbar, fileBrowser *FileBrowser, viewer *Viewer) *MainWindow {
	w := app.NewWindow("Photo Viewer")
	w.Resize(fyne.NewSize(defaultWindowWidth, defaultWindowHeight))

	mw := &MainWindow{
		window:      w,
		toolbar:     toolbar,
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
	leftPanel := mw.fileBrowser.Container()

	split := container.NewHSplit(leftPanel, mw.viewer.Container())
	split.SetOffset(float64(leftPanelWidth) / float64(defaultWindowWidth))

	content := container.NewBorder(mw.toolbar.Container(), nil, nil, nil, split)
	mw.window.SetContent(content)
}
