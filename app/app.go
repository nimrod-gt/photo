package app

import (
	"context"
	"fmt"
	"image"
	"log"
	"os"
	"slices"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/go-gl/glfw/v3.4/glfw"

	"photo/model"
	"photo/service"
	"photo/ui"
)

type Application struct {
	contextMenuItems    ui.ContextMenuItems
	fyneApp             fyne.App
	scanner             *service.Scanner
	colorService        *service.ColorService
	deleter             *service.Deleter
	copier              *service.Copier
	navigator           *service.Navigator
	exifService         *service.ExifService
	imageProvider       *service.ImageProvider
	actionPanel         *ui.ActionPanel
	fileBrowser         *ui.FileBrowser
	viewer              *ui.Viewer
	gridViewer          *ui.GridViewer
	mainWindow          *ui.MainWindow
	deleteDialog        *dialog.ConfirmDialog
	copyDialog          *dialog.ConfirmDialog
	helpDialog          *dialog.CustomDialog
	deleteAllDialog     *dialog.ConfirmDialog
	copyAllDialog       *ui.CopyAllDialog
	copyAllCancel       context.CancelFunc
	filterColors        map[model.ColorLabel]bool
	fullImageSize       func() int
	sortOrder           service.SortOrder
	deleteDialogOpen    bool
	copyDialogOpen      bool
	helpDialogOpen      bool
	deleteAllDialogOpen bool
	copyAllDialogOpen   bool
	sortDescending      bool
	filterFavorite      bool
	gridMode            bool
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
		filterColors:  make(map[model.ColorLabel]bool),
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

	a.colorService.ClearCache()

	a.fyneApp.Preferences().SetString("lastDirectory", dir)

	if ui.HasActiveFilter(a.filterColors, a.filterFavorite) {
		a.filterFavorite = false
		for k := range a.filterColors {
			a.filterColors[k] = false
		}
		a.fileBrowser.SetFilter(a.filterColors, a.filterFavorite)
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

func (a *Application) showPhoto(photo model.Photo) {
	img, err := a.imageProvider.Get(photo.ImagePath, a.fullImageSize())
	if err != nil {
		a.viewer.Clear()
		a.showError("Failed to load image", err)
		return
	}
	a.viewer.ShowPhoto(img)
	a.updateColorIndicators(photo)
	a.prefetchAdjacent()
	a.mainWindow.Window().Canvas().Unfocus()
}

func (a *Application) prefetchAdjacent() {
	var paths []string
	for offset := 1; offset <= 15; offset++ {
		if p, ok := a.navigator.Peek(offset); ok {
			paths = append(paths, p.ImagePath)
		}
	}
	for offset := -1; offset >= -2; offset-- {
		if p, ok := a.navigator.Peek(offset); ok {
			paths = append(paths, p.ImagePath)
		}
	}
	a.imageProvider.Preload(paths, a.fullImageSize(), nil)
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
		if a.gridMode {
			a.gridViewer.ScrollToIndex(idx)
			return
		}
		a.showPhoto(p)
		a.unpinIfMovedAway(p)
	}
}

func (a *Application) handleNext() {
	if a.gridMode {
		return
	}
	if photo, idx, ok := a.navigator.Next(); ok {
		a.showPhoto(photo)
		a.fileBrowser.SelectIndex(idx)
		a.unpinIfMovedAway(photo)
	}
}

func (a *Application) handlePrevious() {
	if a.gridMode {
		return
	}
	if photo, idx, ok := a.navigator.Previous(); ok {
		a.showPhoto(photo)
		a.fileBrowser.SelectIndex(idx)
		a.unpinIfMovedAway(photo)
	}
}

func (a *Application) handleColorToggle(color model.ColorLabel) {
	if a.gridMode {
		return
	}
	photo, ok := a.navigator.Current()
	if !ok {
		return
	}
	if err := a.colorService.ToggleColor(photo, color); err != nil {
		a.showError("Failed to toggle color", err)
		return
	}
	a.updateColorIndicators(photo)
	a.refreshFileBrowserItem(photo)
	if ui.HasActiveFilter(a.filterColors, a.filterFavorite) {
		a.fileBrowser.SetPinnedPath(photo.ImagePath)
		a.reapplyFilter()
	}
}

func (a *Application) refreshFileBrowserItem(photo model.Photo) {
	idx := a.navigator.FindIndex(photo.ImagePath)
	if idx < 0 {
		return
	}
	colors, err := a.colorService.GetColors(photo)
	if err != nil {
		log.Printf("Failed to get colors for %s: %v", photo.Name, err)
		return
	}
	meta := a.fileBrowser.GetMeta(idx)
	a.fileBrowser.RefreshItemMeta(idx, colors, meta.Favorite)
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
	if a.sortOrder == service.SortByTime {
		allMeta := a.fileBrowser.AllMeta()
		dates := make(map[string]time.Time, len(photos))
		for i, p := range photos {
			if i < len(allMeta) {
				dates[p.ImagePath] = allMeta[i].Date
			}
		}
		a.scanner.SortPhotosByDates(photos, dates)
	} else {
		a.scanner.SortPhotos(photos, a.sortOrder)
	}
	if a.sortDescending {
		slices.Reverse(photos)
	}
	a.fileBrowser.SetPhotos(photos)
	a.fileBrowser.SetSortState(a.sortOrder, a.sortDescending)

	filtered := a.fileBrowser.FilteredPhotos()
	a.navigator.SetPhotos(filtered)
	if a.gridMode {
		a.enterGridMode()
		return
	}
	a.showCurrentOrFirst()
}

func (a *Application) unpinIfMovedAway(currentPhoto model.Photo) {
	pinnedPath := a.fileBrowser.PinnedPath()
	if len(pinnedPath) == 0 || !ui.HasActiveFilter(a.filterColors, a.filterFavorite) {
		return
	}
	if currentPhoto.ImagePath == pinnedPath {
		return
	}
	a.fileBrowser.ClearPinnedPath()
	a.reapplyFilter()
}

func (a *Application) handleFilterColor(color model.ColorLabel) {
	a.filterColors[color] = !a.filterColors[color]
	a.fileBrowser.ClearPinnedPath()
	a.reapplyFilter()
}

func (a *Application) handleFilterFavorite() {
	a.filterFavorite = !a.filterFavorite
	a.fileBrowser.ClearPinnedPath()
	a.reapplyFilter()
}

func (a *Application) reapplyFilter() {
	a.fileBrowser.SetFilter(a.filterColors, a.filterFavorite)
	a.syncNavigatorToFiltered()
	if a.gridMode {
		a.enterGridMode()
	}
}

func (a *Application) handleFilteredChanged(photos []model.Photo) {
	a.navigator.SetPhotos(photos)
	if a.gridMode {
		a.enterGridMode()
		return
	}
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
		a.fileBrowser.SelectIndex(a.navigator.CurrentIndex())
	} else {
		a.viewer.Clear()
	}
}

func (a *Application) handleToggleGrid() {
	if a.anyDialogOpen() {
		return
	}
	a.gridMode = !a.gridMode
	a.mainWindow.SetGridMode(a.gridMode)
	if a.gridMode {
		a.enterGridMode()
	} else {
		a.exitGridMode()
	}
}

func (a *Application) enterGridMode() {
	photos := a.fileBrowser.FilteredPhotos()
	meta := a.fileBrowser.FilteredMeta()
	a.gridViewer.SetPhotos(photos, meta)
	a.gridViewer.ScrollToIndex(a.navigator.CurrentIndex())
}

func (a *Application) exitGridMode() {
	a.gridViewer.StopLoading()
	a.showCurrentOrFirst()
}

func (a *Application) handleGridPhotoTapped(index int) {
	a.gridMode = false
	a.mainWindow.SetGridMode(false)
	if photo, _, ok := a.navigator.GoTo(index); ok {
		a.showPhoto(photo)
		a.fileBrowser.SelectIndex(index)
	}
}

func (a *Application) anyDialogOpen() bool {
	return a.deleteDialogOpen || a.copyDialogOpen || a.helpDialogOpen ||
		a.deleteAllDialogOpen || a.copyAllDialogOpen
}

func (a *Application) handleCopyToClipboard() {
	if a.gridMode {
		return
	}
	photo, ok := a.navigator.Current()
	if !ok {
		return
	}
	if err := service.CopyImageToClipboard(photo.ImagePath); err != nil {
		a.showError("Failed to copy to clipboard", err)
		return
	}
	a.mainWindow.ShowNotification("Copied to clipboard")
}

func (a *Application) handleZoomChanged(zoom float32) {
	photo, ok := a.navigator.Current()
	if !ok {
		return
	}
	neededSize := int(float64(a.fullImageSize()) * float64(zoom))
	if img := a.imageProvider.Peek(photo.ImagePath, neededSize); img != nil {
		a.viewer.UpdateImage(img)
		return
	}
	path := photo.ImagePath
	go func() {
		img, err := a.imageProvider.Get(path, neededSize)
		if err != nil {
			log.Printf("Failed to load hi-res image %s: %v", path, err)
			return
		}
		fyne.Do(func() {
			cur, ok := a.navigator.Current()
			if !ok || cur.ImagePath != path {
				return
			}
			a.viewer.UpdateImage(img)
		})
	}()
}

func (a *Application) handleZoomReset() {
	if a.gridMode {
		return
	}
	a.viewer.ResetZoom()
}

func (a *Application) handleZoomIn() {
	if a.gridMode {
		return
	}
	a.viewer.ZoomIn()
}

func (a *Application) handleZoomOut() {
	if a.gridMode {
		return
	}
	a.viewer.ZoomOut()
}

func (a *Application) handleDelete() {
	if a.gridMode {
		return
	}
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
				log.Println("Failed to remove color labels:", err)
			}
			nextPhoto, navIdx, _, hasNext := a.navigator.RemoveCurrent()
			a.fileBrowser.RemovePhoto(photo.ImagePath)
			if hasNext {
				a.showPhoto(nextPhoto)
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
	if a.deleteAllDialogOpen {
		a.deleteAllDialog.Hide()
		a.deleteAllDialogOpen = false
		a.deleteAllDialog = nil
	}
	if a.copyAllDialogOpen {
		if a.copyAllCancel != nil {
			a.copyAllCancel()
		}
		a.copyAllDialog.Hide()
		a.copyAllDialogOpen = false
		a.copyAllDialog = nil
		a.copyAllCancel = nil
	}
}

func (a *Application) handleCopy() {
	if a.gridMode {
		return
	}
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
	copyMode := service.CopyMode(prefs.IntWithFallback("copyMode", int(service.CopyWithRAW)))

	destEntry := ui.NewDestinationEntry(destDir, a.mainWindow.Window())
	modeSelect := ui.NewCopyModeSelect(copyMode)

	content := ui.NewCopyDialogContent(photo.Name, destEntry.Container, modeSelect)

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
			mode := modeSelect.Mode
			prefs.SetString("copyDestination", dest)
			prefs.SetInt("copyMode", int(mode))
			go func() {
				err := a.copier.Copy(photo, dest, mode)
				fyne.Do(func() {
					if err != nil {
						a.showError("Failed to copy photo", err)
						return
					}
					if mode == service.CopyWithRAW && !photo.HasRAW() {
						a.mainWindow.ShowWarning(photo.Name + " copied without RAW (RAW file not found)")
					} else {
						a.mainWindow.ShowNotification(photo.Name + " copied")
					}
				})
			}()
		},
		a.mainWindow.Window(),
	)
	a.copyDialog.Show()
}

func (a *Application) handleDeleteAll() {
	if a.gridMode {
		return
	}
	filtered := a.fileBrowser.FilteredPhotos()
	if len(filtered) == 0 {
		return
	}

	content, rawCheck := ui.NewDeleteAllDialogContent(len(filtered))

	a.deleteAllDialogOpen = true
	a.deleteAllDialog = dialog.NewCustomConfirm("Delete All",
		"Delete", "Cancel",
		content,
		func(confirmed bool) {
			a.deleteAllDialogOpen = false
			a.deleteAllDialog = nil
			if !confirmed {
				return
			}
			includeRAW := rawCheck.Checked
			deleted := 0
			var deletedPhotos []model.Photo
			for _, photo := range filtered {
				if err := a.deleter.DeleteWithOption(photo, includeRAW); err != nil {
					a.showError("Failed to delete "+photo.Name, err)
					continue
				}
				deleted++
				deletedPhotos = append(deletedPhotos, photo)
			}
			if err := a.colorService.RemoveMultipleColors(deletedPhotos); err != nil {
				a.showError("Failed to remove color labels", err)
			}
			paths := make(map[string]bool, len(deletedPhotos))
			for _, p := range deletedPhotos {
				paths[p.ImagePath] = true
			}
			a.fileBrowser.RemovePhotos(paths)
			newFiltered := a.fileBrowser.FilteredPhotos()
			a.navigator.SetPhotos(newFiltered)
			if p, navIdx, ok := a.navigator.GoTo(0); ok {
				a.showPhoto(p)
				a.fileBrowser.SelectIndex(navIdx)
			} else {
				a.viewer.Clear()
			}
			a.mainWindow.ShowNotification(fmt.Sprintf("Deleted %d photos", deleted))
		},
		a.mainWindow.Window(),
	)
	a.deleteAllDialog.Show()
}

func (a *Application) handleCopyAll() {
	if a.gridMode {
		return
	}
	filtered := a.fileBrowser.FilteredPhotos()
	if len(filtered) == 0 {
		return
	}

	prefs := a.fyneApp.Preferences()
	destDir := prefs.String("copyDestination")
	copyMode := service.CopyMode(prefs.IntWithFallback("copyMode", int(service.CopyWithRAW)))

	ctx, cancel := context.WithCancel(context.Background())
	a.copyAllCancel = cancel

	a.copyAllDialogOpen = true
	a.copyAllDialog = ui.NewCopyAllDialog(len(filtered), destDir, copyMode, a.mainWindow.Window(),
		func() {
			dest := a.copyAllDialog.DestDir()
			if len(dest) == 0 {
				a.mainWindow.ShowError("No destination folder selected")
				a.copyAllDialog.Finish()
				return
			}
			mode := a.copyAllDialog.CopyMode()
			prefs.SetString("copyDestination", dest)
			prefs.SetInt("copyMode", int(mode))

			go func() {
				total := len(filtered)
				copied := 0
				skipped := 0
				for i, photo := range filtered {
					if err := a.copier.CopyWithContext(ctx, photo, dest, mode); err != nil {
						if ctx.Err() != nil {
							fyne.Do(func() {
								a.mainWindow.ShowWarning(fmt.Sprintf("Copy cancelled after %d/%d photos", copied, total))
								if a.copyAllDialog != nil {
									a.copyAllDialog.Hide()
									a.copyAllDialogOpen = false
									a.copyAllDialog = nil
								}
								a.copyAllCancel = nil
							})
							return
						}
						log.Printf("Failed to copy %s: %v", photo.Name, err)
						skipped++
						continue
					}
					copied++
					progress := float64(i+1) / float64(total)
					fyne.Do(func() {
						if a.copyAllDialog != nil {
							a.copyAllDialog.SetProgress(progress)
						}
					})
				}
				fyne.Do(func() {
					if a.copyAllDialog != nil {
						a.copyAllDialog.Finish()
						a.copyAllDialogOpen = false
						a.copyAllDialog = nil
					}
					a.copyAllCancel = nil
					if skipped > 0 {
						a.mainWindow.ShowWarning(fmt.Sprintf("Copied %d/%d photos (%d skipped)", copied, total, skipped))
					} else {
						a.mainWindow.ShowNotification(fmt.Sprintf("Copied %d/%d photos", copied, total))
					}
				})
			}()
		},
		func() {
			if a.copyAllCancel != nil {
				a.copyAllCancel()
			}
			if a.copyAllDialog != nil {
				a.copyAllDialog.Hide()
			}
			a.copyAllDialogOpen = false
			a.copyAllDialog = nil
			a.copyAllCancel = nil
		},
	)
	a.copyAllDialog.Show()
}
