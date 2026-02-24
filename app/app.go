package app

import (
	"fmt"
	"image"
	"log"
	"os"
	"slices"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/go-gl/glfw/v3.3/glfw"

	"photo/model"
	"photo/service"
	"photo/ui"
)

var defaultMaxImageSize = image.Point{X: 3840, Y: 2160}

type Application struct {
	fyneApp fyne.App

	scanner        *service.Scanner
	exifService    *service.ExifService
	colorService   *service.ColorService
	deleter        *service.Deleter
	copier         *service.Copier
	navigator      *service.Navigator
	imageCache     *service.ImageCache
	metadataLoader *service.MetadataLoader

	actionPanel      *ui.ActionPanel
	fileBrowser      *ui.FileBrowser
	viewer           *ui.Viewer
	mainWindow       *ui.MainWindow
	contextMenuItems ui.ContextMenuItems
	deleteDialog     *dialog.ConfirmDialog
	deleteDialogOpen bool
	copyDialog       *dialog.ConfirmDialog
	copyDialogOpen   bool
	helpDialog       *dialog.CustomDialog
	helpDialogOpen   bool
	sortOrder        service.SortOrder
	sortDescending   bool
	filterColors     map[model.ColorLabel]bool
	filterFavorite   bool
}

func New() *Application {
	exifService := service.NewExifService()
	return &Application{
		scanner:        service.NewScanner(),
		exifService:    exifService,
		colorService:   service.NewColorService(),
		deleter:        service.NewDeleter(),
		copier:         service.NewCopier(),
		navigator:      service.NewNavigator(),
		metadataLoader: service.NewMetadataLoader(exifService),
		filterColors:   make(map[model.ColorLabel]bool),
	}
}

func (a *Application) Run() {
	a.fyneApp = fyneapp.NewWithID("com.photo.viewer")
	fyneApp := a.fyneApp
	fyneApp.Settings().SetTheme(ui.NewDarkTheme())

	a.actionPanel = ui.NewActionPanel(ui.ActionPanelCallbacks{
		OnFavorite: a.handleFavorite,
		OnRed:      func() { a.handleColorToggle(model.ColorRed) },
		OnGreen:    func() { a.handleColorToggle(model.ColorGreen) },
		OnBlue:     func() { a.handleColorToggle(model.ColorBlue) },
		OnDelete:   a.handleDelete,
	})

	a.fileBrowser = ui.NewFileBrowser(a.scanner, a.metadataLoader, a.colorService, ui.FileBrowserCallbacks{
		OnPhotoSelected:     a.handlePhotoSelected,
		OnDirectorySelected: a.handleDirectorySelected,
		OnChooseFolder:      a.handleChooseFolder,
		OnSortBy:            a.handleSortBy,
		OnFilterRed:         func() { a.handleFilterColor(model.ColorRed) },
		OnFilterGreen:       func() { a.handleFilterColor(model.ColorGreen) },
		OnFilterBlue:        func() { a.handleFilterColor(model.ColorBlue) },
		OnFilterFavorite:    a.handleFilterFavorite,
		OnFilteredChanged:   a.handleFilteredChanged,
	})

	a.viewer = ui.NewViewer(ui.ViewerCallbacks{
		OnTapped:          a.handleNext,
		OnSecondaryTapped: a.handleSecondaryTap,
	})

	a.contextMenuItems = ui.NewContextMenu(ui.ContextMenuCallbacks{
		OnFavorite: a.handleFavorite,
		OnRed:      func() { a.handleColorToggle(model.ColorRed) },
		OnGreen:    func() { a.handleColorToggle(model.ColorGreen) },
		OnBlue:     func() { a.handleColorToggle(model.ColorBlue) },
		OnDelete:   a.handleDelete,
	})

	notifier := ui.NewNotifier()
	a.mainWindow = ui.NewMainWindow(fyneApp, a.actionPanel, a.fileBrowser, a.viewer, notifier)

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
	})

	a.imageCache = service.NewImageCache(monitorSize())

	a.loadInitialDirectory()

	if prefs := fyneApp.Preferences(); !prefs.BoolWithFallback("helpShown", false) {
		prefs.SetBool("helpShown", true)
		a.handleHelp()
	}

	a.mainWindow.Show()
}

func monitorSize() (size image.Point) {
	size = defaultMaxImageSize
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
	a.imageCache.Clear()

	photos, err := a.scanner.ScanDirectory(dir)
	if err != nil {
		a.showError("Failed to scan directory", err)
		return
	}

	a.fyneApp.Preferences().SetString("lastDirectory", dir)

	a.scanner.SortPhotos(photos, a.sortOrder)
	if a.sortDescending {
		reversePhotos(photos)
	}
	a.fileBrowser.SetPhotos(photos)

	filtered := a.fileBrowser.FilteredPhotos()
	a.navigator.SetPhotos(filtered)
	a.showCurrentOrFirst()
}

func (a *Application) showPhoto(photo model.Photo) {
	img, err := a.imageCache.Load(photo.ImagePath)
	if err != nil {
		a.viewer.Clear()
		a.showError("Failed to load image", err)
		return
	}
	a.viewer.ShowPhoto(img)
	a.updateColorIndicators(photo)
	a.updateFavoriteState(photo)
	a.prefetchAdjacent()
	a.mainWindow.Window().Canvas().Unfocus()
}

func (a *Application) prefetchAdjacent() {
	var keep []string
	if cur, ok := a.navigator.Current(); ok {
		keep = append(keep, cur.ImagePath)
	}
	for _, offset := range []int{-2, -1, 1, 2} {
		if p, ok := a.navigator.Peek(offset); ok {
			keep = append(keep, p.ImagePath)
			a.imageCache.Prefetch(p.ImagePath)
		}
	}
	a.imageCache.EvictExcept(keep)
}

func (a *Application) updateColorIndicators(photo model.Photo) {
	colors, err := a.colorService.GetColors(photo)
	if err != nil {
		a.showError("Failed to get colors", err)
		return
	}
	a.viewer.SetColorIndicators(colors)
	a.updateColorButtonStates(colors)
}

func (a *Application) updateColorButtonStates(colors []model.ColorLabel) {
	activeSet := ui.ColorSet(colors)
	a.actionPanel.SetColorActive(model.ColorRed, activeSet[model.ColorRed])
	a.actionPanel.SetColorActive(model.ColorGreen, activeSet[model.ColorGreen])
	a.actionPanel.SetColorActive(model.ColorBlue, activeSet[model.ColorBlue])
	a.contextMenuItems.Red.Checked = activeSet[model.ColorRed]
	a.contextMenuItems.Green.Checked = activeSet[model.ColorGreen]
	a.contextMenuItems.Blue.Checked = activeSet[model.ColorBlue]
}

func (a *Application) handlePhotoSelected(photo model.Photo) {
	idx := a.navigator.FindIndex(photo.ImagePath)
	if idx < 0 {
		return
	}
	if p, _, ok := a.navigator.GoTo(idx); ok {
		a.showPhoto(p)
	}
}

func (a *Application) handleNext() {
	if photo, idx, ok := a.navigator.Next(); ok {
		a.showPhoto(photo)
		a.fileBrowser.SelectIndex(idx)
	}
}

func (a *Application) handlePrevious() {
	if photo, idx, ok := a.navigator.Previous(); ok {
		a.showPhoto(photo)
		a.fileBrowser.SelectIndex(idx)
	}
}

func (a *Application) updateFavoriteState(photo model.Photo) {
	a.actionPanel.SetFavoriteEnabled(photo.IsJPEG())
	a.contextMenuItems.Favorite.Disabled = !photo.IsJPEG()

	if !photo.IsJPEG() {
		a.actionPanel.SetFavoriteActive(false)
		a.contextMenuItems.Favorite.Checked = false
		return
	}

	rating, _ := a.exifService.GetRating(photo.ImagePath)
	active := rating > 0
	a.actionPanel.SetFavoriteActive(active)
	a.contextMenuItems.Favorite.Checked = active
}

func (a *Application) handleFavorite() {
	photo, ok := a.navigator.Current()
	if !ok {
		return
	}
	if !photo.IsJPEG() {
		return
	}
	if err := a.exifService.ToggleFavorite(photo.ImagePath); err != nil {
		a.showError("Failed to toggle favorite", err)
		return
	}
	a.updateFavoriteState(photo)
	a.refreshFileBrowserItem(photo)
	if a.hasActiveFilter() {
		a.reapplyFilter()
	}
}

func (a *Application) handleColorToggle(color model.ColorLabel) {
	photo, ok := a.navigator.Current()
	if !ok {
		return
	}
	if err := a.colorService.ToggleColor(photo, color); err != nil {
		a.showError("Failed to toggle color", err)
	}
	a.updateColorIndicators(photo)
	a.refreshFileBrowserItem(photo)
	if a.hasActiveFilter() {
		a.reapplyFilter()
	}
}

func (a *Application) refreshFileBrowserItem(photo model.Photo) {
	idx := a.navigator.FindIndex(photo.ImagePath)
	if idx < 0 {
		return
	}
	colors, _ := a.colorService.GetColors(photo)
	rating, _ := a.exifService.GetRating(photo.ImagePath)
	a.fileBrowser.RefreshItemMeta(idx, colors, rating > 0)
}

func (a *Application) handleSecondaryTap(pos fyne.Position) {
	ui.ShowContextMenu(a.contextMenuItems.Menu, a.mainWindow.Window().Canvas(), pos)
}

func (a *Application) handleSortBy(order service.SortOrder) {
	if a.sortOrder == order {
		a.sortDescending = !a.sortDescending
	} else {
		a.sortOrder = order
		a.sortDescending = false
	}
	a.resortPhotos()
}

func (a *Application) handleSortToggle() {
	if a.sortDescending {
		if a.sortOrder == service.SortByName {
			a.sortOrder = service.SortByTime
		} else {
			a.sortOrder = service.SortByName
		}
		a.sortDescending = false
	} else {
		a.sortDescending = true
	}
	a.resortPhotos()
}

func (a *Application) resortPhotos() {
	photos := a.fileBrowser.AllPhotos()
	a.scanner.SortPhotos(photos, a.sortOrder)
	if a.sortDescending {
		reversePhotos(photos)
	}
	a.fileBrowser.SetPhotos(photos)
	a.fileBrowser.SetSortState(a.sortOrder, a.sortDescending)

	filtered := a.fileBrowser.FilteredPhotos()
	a.navigator.SetPhotos(filtered)
	a.showCurrentOrFirst()
}

func reversePhotos(photos []model.Photo) {
	slices.Reverse(photos)
}

func (a *Application) handleFilterColor(color model.ColorLabel) {
	a.filterColors[color] = !a.filterColors[color]
	a.reapplyFilter()
}

func (a *Application) handleFilterFavorite() {
	a.filterFavorite = !a.filterFavorite
	a.reapplyFilter()
}

func (a *Application) reapplyFilter() {
	a.fileBrowser.SetFilter(a.filterColors, a.filterFavorite)
	a.syncNavigatorToFiltered()
}

func (a *Application) handleFilteredChanged(photos []model.Photo) {
	a.navigator.SetPhotos(photos)
	a.showCurrentOrFirst()
}

func (a *Application) syncNavigatorToFiltered() {
	var currentPath string
	if cur, ok := a.navigator.Current(); ok {
		currentPath = cur.ImagePath
	}

	filtered := a.fileBrowser.FilteredPhotos()
	a.navigator.SetPhotos(filtered)

	if len(currentPath) > 0 {
		idx := a.navigator.FindIndex(currentPath)
		if idx >= 0 {
			if p, _, ok := a.navigator.GoTo(idx); ok {
				a.showPhoto(p)
				a.fileBrowser.SelectIndex(idx)
				return
			}
		}
	}

	a.showCurrentOrFirst()
}

func (a *Application) showCurrentOrFirst() {
	if photo, ok := a.navigator.Current(); ok {
		a.showPhoto(photo)
		a.fileBrowser.SelectIndex(0)
	} else {
		a.viewer.Clear()
	}
}

func (a *Application) hasActiveFilter() bool {
	if a.filterFavorite {
		return true
	}
	for _, v := range a.filterColors {
		if v {
			return true
		}
	}
	return false
}

func (a *Application) handleDelete() {
	if a.deleteDialogOpen {
		a.deleteDialog.Confirm()
		return
	}

	photo, ok := a.navigator.Current()
	if !ok {
		return
	}

	a.deleteDialogOpen = true
	message := "Delete " + photo.Name + "?"
	if photo.HasRAW() {
		message += " This will also delete the RAW pair."
	}
	a.deleteDialog = dialog.NewCustomConfirm("Delete Photo",
		"Delete (D)", "Cancel (N)",
		widget.NewLabel(message),
		func(confirmed bool) {
			a.deleteDialogOpen = false
			a.deleteDialog = nil
			if !confirmed {
				return
			}
			if err := a.deleter.Delete(photo); err != nil {
				a.showError("Failed to delete photo", err)
				return
			}
			if err := a.colorService.RemoveColors(photo); err != nil {
				a.showError("Failed to remove color labels", err)
			}
			prevIdx := a.navigator.FindIndex(photo.ImagePath)
			a.fileBrowser.RemovePhoto(photo.ImagePath)
			filtered := a.fileBrowser.FilteredPhotos()
			a.navigator.SetPhotos(filtered)

			if prevIdx >= len(filtered) {
				prevIdx = len(filtered) - 1
			}
			if prevIdx < 0 {
				prevIdx = 0
			}
			if p, navIdx, ok := a.navigator.GoTo(prevIdx); ok {
				a.showPhoto(p)
				a.fileBrowser.SelectIndex(navIdx)
			} else {
				a.viewer.Clear()
			}
		},
		a.mainWindow.Window(),
	)
	a.deleteDialog.Show()
}

func (a *Application) handleHelp() {
	if a.helpDialogOpen {
		a.helpDialog.Hide()
		a.helpDialogOpen = false
		a.helpDialog = nil
		return
	}
	a.helpDialog = ui.NewHelp(a.mainWindow.Window())
	a.helpDialog.SetOnClosed(func() {
		a.helpDialogOpen = false
		a.helpDialog = nil
	})
	a.helpDialogOpen = true
	a.helpDialog.Show()
}

func (a *Application) handleCancel() {
	if a.deleteDialogOpen {
		a.deleteDialog.Hide()
		a.deleteDialogOpen = false
		a.deleteDialog = nil
	}
	if a.copyDialogOpen {
		a.copyDialog.Hide()
		a.copyDialogOpen = false
		a.copyDialog = nil
	}
}

func (a *Application) handleCopy() {
	if a.copyDialogOpen {
		a.copyDialog.Confirm()
		return
	}

	photo, ok := a.navigator.Current()
	if !ok {
		return
	}

	prefs := a.fyneApp.Preferences()
	destDir := prefs.String("copyDestination")
	includeRAW := prefs.BoolWithFallback("copyIncludeRAW", true)

	destEntry := ui.NewDestinationEntry(destDir, a.mainWindow.Window())
	rawCheck := ui.NewRawCheck(includeRAW)

	content := ui.NewCopyDialogContent(photo.Name, destEntry.Container, rawCheck)

	a.copyDialogOpen = true
	a.copyDialog = dialog.NewCustomConfirm("Copy Photo", "Copy (C)", "Cancel (N)",
		content,
		func(confirmed bool) {
			a.copyDialogOpen = false
			a.copyDialog = nil
			if !confirmed {
				return
			}
			dest := destEntry.Text()
			if len(dest) == 0 {
				a.mainWindow.ShowError("No destination folder selected")
				return
			}
			prefs.SetString("copyDestination", dest)
			prefs.SetBool("copyIncludeRAW", rawCheck.Checked)
			if err := a.copier.Copy(photo, dest, rawCheck.Checked); err != nil {
				a.showError("Failed to copy photo", err)
				return
			}
			if rawCheck.Checked && !photo.HasRAW() {
				a.mainWindow.ShowWarning(photo.Name + " copied without RAW (RAW file not found)")
			} else {
				a.mainWindow.ShowNotification(photo.Name + " copied")
			}
		},
		a.mainWindow.Window(),
	)
	a.copyDialog.Show()
}
