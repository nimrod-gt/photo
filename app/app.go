package app

import (
	"fmt"
	"image"
	"log"
	"os"
	"slices"
	"sync"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
	"github.com/go-gl/glfw/v3.4/glfw"

	"photo/model"
	"photo/service"
	"photo/ui"
)

const largeFolderWarnThreshold = 10000

type Application struct {
	contextMenuItems ui.ContextMenuItems
	fyneApp          fyne.App
	scanner          *service.Scanner
	colorService     *service.ColorService
	deleter          *service.Deleter
	copier           *service.Copier
	navigator        *service.Navigator
	exifService      *service.ExifService
	imageProvider    *service.ImageProvider
	actionPanel      *ui.ActionPanel
	fileBrowser      *ui.FileBrowser
	viewer           *ui.Viewer
	gridViewer       *ui.GridViewer
	mainWindow       *ui.MainWindow
	dialogs          dialogManager
	fullImageSize    func() int
	sortOrder        service.SortOrder
	sortDescending   bool
	gridMode         bool
}

func New() *Application {
	exifService := service.NewExifService()
	return &Application{
		scanner:       service.NewScanner(),
		colorService:  service.NewColorService(),
		deleter:       service.NewDeleter(),
		copier:        service.NewCopier(),
		navigator:     service.NewNavigator(),
		exifService:   exifService,
		imageProvider: service.NewImageProvider(exifService),
	}
}

func (a *Application) Run() {
	a.fyneApp = fyneapp.NewWithID("com.photo.viewer")
	fyneApp := a.fyneApp
	fyneApp.Settings().SetTheme(ui.NewDarkTheme())

	a.actionPanel = ui.NewActionPanel(ui.ActionPanelCallbacks{
		OnRed:    func() { a.handleColorToggle(model.ColorRed) },
		OnGreen:  func() { a.handleColorToggle(model.ColorGreen) },
		OnBlue:   func() { a.handleColorToggle(model.ColorBlue) },
		OnDelete: a.handleDelete,
	})

	a.fileBrowser = ui.NewFileBrowser(a.scanner, a.imageProvider, a.colorService, ui.FileBrowserCallbacks{
		OnPhotoSelected:     a.handlePhotoSelected,
		OnDirectorySelected: a.handleDirectorySelected,
		OnChooseFolder:      a.handleChooseFolder,
		OnSortBy:            a.handleSortBy,
		OnFilterRed:         func() { a.handleFilterColor(model.ColorRed) },
		OnFilterGreen:       func() { a.handleFilterColor(model.ColorGreen) },
		OnFilterBlue:        func() { a.handleFilterColor(model.ColorBlue) },
		OnFilterFavorite:    a.handleFilterFavorite,
		OnFilteredChanged:   a.handleFilteredChanged,
		OnDeleteAll:         a.handleDeleteAll,
		OnCopyAll:           a.handleCopyAll,
		OnUnselectAll:       a.handleUnselectAll,
		OnHelp:              a.handleHelp,
	})

	a.viewer = ui.NewViewer(ui.ViewerCallbacks{
		OnTapped:          a.handleNext,
		OnSecondaryTapped: a.handleSecondaryTap,
		OnZoomChanged:     a.handleZoomChanged,
	})

	a.fullImageSize = sync.OnceValue(func() int {
		s := monitorSize()
		return max(s.X, s.Y)
	})

	a.gridViewer = ui.NewGridViewer(a.imageProvider, ui.GridViewerCallbacks{
		OnPhotoTapped: a.handleGridPhotoTapped,
	})

	a.contextMenuItems = ui.NewContextMenu(ui.ContextMenuCallbacks{
		OnRed:           func() { a.handleColorToggle(model.ColorRed) },
		OnGreen:         func() { a.handleColorToggle(model.ColorGreen) },
		OnBlue:          func() { a.handleColorToggle(model.ColorBlue) },
		OnDelete:        a.handleDelete,
		OnCopyClipboard: a.handleCopyToClipboard,
	})

	notifier := ui.NewNotifier()
	a.mainWindow = ui.NewMainWindow(fyneApp, a.actionPanel, a.fileBrowser, a.viewer, a.gridViewer, notifier)

	ui.SetupShortcuts(a.mainWindow.Window().Canvas(), ui.ShortcutCallbacks{
		OnRed:            func() { a.handleColorToggle(model.ColorRed) },
		OnGreen:          func() { a.handleColorToggle(model.ColorGreen) },
		OnBlue:           func() { a.handleColorToggle(model.ColorBlue) },
		OnDelete:         a.handleDelete,
		OnCopy:           a.handleCopy,
		OnCancel:         a.handleCancel,
		OnNext:           a.handleNext,
		OnPrevious:       a.handlePrevious,
		OnSort:           a.handleSortToggle,
		OnFilterRed:      func() { a.handleFilterColor(model.ColorRed) },
		OnFilterGreen:    func() { a.handleFilterColor(model.ColorGreen) },
		OnFilterBlue:     func() { a.handleFilterColor(model.ColorBlue) },
		OnFilterFavorite: a.handleFilterFavorite,
		OnHelp:           a.handleHelp,
		OnCopyClipboard:  a.handleCopyToClipboard,
		OnToggleGrid:     a.handleToggleGrid,
		OnZoomReset:      a.handleZoomReset,
		OnZoomIn:         a.handleZoomIn,
		OnZoomOut:        a.handleZoomOut,
	})

	a.loadInitialDirectory()

	if prefs := fyneApp.Preferences(); !prefs.BoolWithFallback("helpShown", false) {
		prefs.SetBool("helpShown", true)
		a.handleHelp()
	}

	a.mainWindow.Show()
}

func monitorSize() (size image.Point) {
	size = image.Point{X: 3840, Y: 2160}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("monitorSize: recovered from panic: %v", r)
		}
	}()
	mon := glfw.GetPrimaryMonitor()
	if mon == nil {
		return
	}
	mode := mon.GetVideoMode()
	if mode == nil {
		return
	}
	return image.Point{X: mode.Width, Y: mode.Height}
}

func (a *Application) showError(msg string, err error) {
	full := fmt.Sprintf("%s: %v", msg, err)
	log.Println(full)
	a.mainWindow.ShowError(full)
}

func (a *Application) handleDirectorySelected(dir string) {
	a.loadDirectory(dir)
}

func (a *Application) handleChooseFolder() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		root := uri.Path()
		a.fyneApp.Preferences().SetString("lastRoot", root)
		a.fileBrowser.SetRoot(root)
		a.loadDirectory(root)
	}, a.mainWindow.Window())
}

func isValidDirectory(path string) bool {
	if len(path) == 0 {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (a *Application) loadInitialDirectory() {
	lastRoot := a.fyneApp.Preferences().String("lastRoot")
	lastDir := a.fyneApp.Preferences().String("lastDirectory")

	if isValidDirectory(lastRoot) {
		a.fileBrowser.SetRoot(lastRoot)
		if isValidDirectory(lastDir) {
			a.fileBrowser.OpenDirectory(lastDir)
			a.loadDirectory(lastDir)
		} else {
			a.loadDirectory(lastRoot)
		}
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		a.showError("Failed to get home directory", err)
		return
	}
	a.fyneApp.Preferences().SetString("lastRoot", home)
	a.fileBrowser.SetRoot(home)
	a.loadDirectory(home)
}

func (a *Application) loadDirectory(dir string) {
	photos, err := a.scanner.ScanDirectory(dir)
	if err != nil {
		a.showError("Failed to scan directory", err)
		return
	}

	if len(photos) >= largeFolderWarnThreshold {
		dialog.ShowInformation("Large Folder",
			fmt.Sprintf("This folder contains %d photos.\nThe app may work slowly.", len(photos)),
			a.mainWindow.Window())
	}

	a.colorService.ClearCache()

	a.fyneApp.Preferences().SetString("lastDirectory", dir)

	if a.fileBrowser.HasFilter() {
		a.fileBrowser.ClearFilter()
	}

	a.scanner.SortPhotos(photos, a.sortOrder)
	if a.sortDescending {
		slices.Reverse(photos)
	}
	a.fileBrowser.SetPhotos(photos)

	filtered := a.fileBrowser.FilteredPhotos()
	a.navigator.SetPhotos(filtered)
	if a.gridMode {
		a.enterGridMode()
		return
	}
	a.showCurrentOrFirst()
}
