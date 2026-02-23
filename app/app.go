package app

import (
	"fmt"
	"image"
	"log"
	"os"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
	"github.com/go-gl/glfw/v3.3/glfw"

	"photo/model"
	"photo/service"
	"photo/ui"
)

var defaultMaxImageSize = image.Point{X: 3840, Y: 2160}

type Application struct {
	scanner        *service.Scanner
	exifService    *service.ExifService
	colorService   *service.ColorService
	deleter        *service.Deleter
	navigator      *service.Navigator
	imageCache     *service.ImageCache
	metadataLoader *service.MetadataLoader

	actionPanel      *ui.ActionPanel
	fileBrowser      *ui.FileBrowser
	viewer           *ui.Viewer
	mainWindow       *ui.MainWindow
	contextMenuItems ui.ContextMenuItems
	deleteDialogOpen bool
}

func New() *Application {
	exifService := service.NewExifService()
	return &Application{
		scanner:        service.NewScanner(),
		exifService:    exifService,
		colorService:   service.NewColorService(),
		deleter:        service.NewDeleter(),
		navigator:      service.NewNavigator(),
		metadataLoader: service.NewMetadataLoader(exifService),
	}
}

func (a *Application) Run() {
	fyneApp := fyneapp.NewWithID("com.photo.viewer")
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
		OnFavorite: a.handleFavorite,
		OnRed:      func() { a.handleColorToggle(model.ColorRed) },
		OnGreen:    func() { a.handleColorToggle(model.ColorGreen) },
		OnBlue:     func() { a.handleColorToggle(model.ColorBlue) },
		OnDelete:   a.handleDelete,
		OnNext:     a.handleNext,
		OnPrevious: a.handlePrevious,
	})

	a.imageCache = service.NewImageCache(monitorSize())

	a.loadInitialDirectory()
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
		a.fileBrowser.SetRoot(root)
		a.loadDirectory(root)
	}, a.mainWindow.Window())
}

func (a *Application) loadInitialDirectory() {
	home, err := os.UserHomeDir()
	if err != nil {
		a.showError("Failed to get home directory", err)
		return
	}
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

	a.scanner.SortPhotos(photos, service.SortByName)
	a.navigator.SetPhotos(photos)
	a.fileBrowser.SetPhotos(photos)

	if photo, ok := a.navigator.Current(); ok {
		a.showPhoto(photo)
	} else {
		a.viewer.Clear()
	}
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

func (a *Application) handleDelete() {
	if a.deleteDialogOpen {
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
	dialog.ShowConfirm("Delete Photo",
		message,
		func(confirmed bool) {
			a.deleteDialogOpen = false
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
			next, idx, photos, ok := a.navigator.RemoveCurrent()
			if ok {
				a.showPhoto(next)
			} else {
				a.viewer.Clear()
			}
			a.fileBrowser.SetPhotos(photos)
			a.fileBrowser.SelectIndex(idx)
		},
		a.mainWindow.Window(),
	)
}
