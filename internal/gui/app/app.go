package app

import (
	"fmt"
	"log"
	"os"
	"slices"
	"sync"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"

	"photo/internal/core/imaging"
	"photo/internal/core/library"
	"photo/internal/core/model"
	"photo/internal/core/tags"
	"photo/internal/gui/nativewin"
	"photo/internal/gui/ui"
)

const largeFolderWarnThreshold = 10000

type Application struct {
	contextMenuItems ui.ContextMenuItems
	fyneApp          fyne.App
	scanner          *library.Scanner
	colorService     *library.ColorService
	deleter          *library.Deleter
	copier           *library.Copier
	navigator        *library.Navigator
	exifService      *imaging.ExifService
	imageProvider    *imaging.Provider
	tagger           *tags.Tagger
	actionPanel      *ui.ActionPanel
	fileBrowser      *ui.FileBrowser
	viewer           *ui.Viewer
	gridViewer       *ui.GridViewer
	mainWindow       *ui.MainWindow
	dialogs          dialogManager
	fullImageSize    func() int
	sortOrder        library.SortOrder
	sortDescending   bool
	gridMode         bool
	showTags         bool
	tagGeneration    int
}

func New() *Application {
	exifService := imaging.NewExifService()
	return &Application{
		scanner:       library.NewScanner(),
		colorService:  library.NewColorService(),
		deleter:       library.NewDeleter(),
		copier:        library.NewCopier(),
		navigator:     library.NewNavigator(),
		exifService:   exifService,
		imageProvider: imaging.NewProvider(exifService),
		tagger:        tags.NewTagger(),
	}
}

// FyneApp.toml carries the same declaration, and a packaged build carries it in
// the metadata the fyne tool generates, but Fyne only reads the file next to the
// working directory or the executable, which a run from an IDE satisfies neither
// of. The flag is added to whatever metadata is already there rather than set on
// its own, because SetMetadata replaces the struct whole and would drop the name,
// the identifier and the icon a packaged build ships with.
//
// It has to happen before the first widget is built: the flag is read once, from
// the thread check that every widget runs.
func declareThreadingMigration(fyneApp fyne.App) {
	meta := fyneApp.Metadata()
	if meta.Migrations == nil {
		meta.Migrations = map[string]bool{"fyneDo": true}
		fyneapp.SetMetadata(meta)
		return
	}
	meta.Migrations["fyneDo"] = true
}

func (a *Application) Run() {
	a.fyneApp = fyneapp.NewWithID("com.photo.viewer")
	fyneApp := a.fyneApp
	declareThreadingMigration(fyneApp)
	fyneApp.Settings().SetTheme(ui.NewDarkTheme())

	a.actionPanel = ui.NewActionPanel(ui.ActionPanelCallbacks{
		OnFavorite: a.handleFavorite,
		OnRed:      func() { a.handleColorToggle(model.ColorRed) },
		OnGreen:    func() { a.handleColorToggle(model.ColorGreen) },
		OnBlue:     func() { a.handleColorToggle(model.ColorBlue) },
		OnTags:     a.handleTags,
		OnDelete:   a.handleDelete,
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
		OnMetaLoaded:        a.handleMetaLoaded,
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
		s := nativewin.MonitorSize()
		return max(s.X, s.Y)
	})

	a.gridViewer = ui.NewGridViewer(a.imageProvider, ui.GridViewerCallbacks{
		OnPhotoTapped: a.handleGridPhotoTapped,
	})

	a.contextMenuItems = ui.NewContextMenu(ui.ContextMenuCallbacks{
		OnFavorite:      a.handleFavorite,
		OnRed:           func() { a.handleColorToggle(model.ColorRed) },
		OnGreen:         func() { a.handleColorToggle(model.ColorGreen) },
		OnBlue:          func() { a.handleColorToggle(model.ColorBlue) },
		OnDelete:        a.handleDelete,
		OnCopyClipboard: a.handleCopyToClipboard,
		OnTags:          a.handleTags,
	})

	notifier := ui.NewNotifier()
	a.mainWindow = ui.NewMainWindow(fyneApp, a.actionPanel, a.fileBrowser, a.viewer, a.gridViewer, notifier)

	// The window has no native handle to maximize before the driver loop has
	// created it, which the loop does on its way to the first frame.
	fyneApp.Lifecycle().SetOnStarted(a.mainWindow.Maximize)

	ui.SetupShortcuts(a.mainWindow.Window().Canvas(), ui.ShortcutCallbacks{
		OnFavorite:       a.handleFavorite,
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
		OnTags:           a.handleTags,
		OnToggleTags:     a.handleToggleTags,
	})

	a.restoreSortOrder()
	a.restoreTagVisibility()
	a.loadInitialDirectory()

	if prefs := fyneApp.Preferences(); !prefs.BoolWithFallback("helpShown", false) {
		prefs.SetBool("helpShown", true)
		a.handleHelp()
	}

	a.mainWindow.Show()
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
