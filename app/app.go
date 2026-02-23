package app

import (
	"fmt"
	"log"
	"os"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"

	"photo/model"
	"photo/service"
	"photo/ui"
)

type Application struct {
	scanner      *service.Scanner
	exifService  *service.ExifService
	colorService *service.ColorService
	deleter      *service.Deleter
	navigator    *service.Navigator

	actionPanel      *ui.ActionPanel
	fileBrowser      *ui.FileBrowser
	viewer           *ui.Viewer
	mainWindow       *ui.MainWindow
	contextMenu      *fyne.Menu
	favoriteMenuItem *fyne.MenuItem
	deleteDialogOpen bool
}

func New() *Application {
	return &Application{
		scanner:      service.NewScanner(),
		exifService:  service.NewExifService(),
		colorService: service.NewColorService(),
		deleter:      service.NewDeleter(),
		navigator:    service.NewNavigator(),
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

	a.fileBrowser = ui.NewFileBrowser(a.scanner, ui.FileBrowserCallbacks{
		OnPhotoSelected:     a.handlePhotoSelected,
		OnDirectorySelected: a.handleDirectorySelected,
		OnChooseFolder:      a.handleChooseFolder,
	})

	a.viewer = ui.NewViewer(ui.ViewerCallbacks{
		OnTapped:          a.handleNext,
		OnSecondaryTapped: a.handleSecondaryTap,
	})

	a.contextMenu, a.favoriteMenuItem = ui.NewContextMenu(ui.ContextMenuCallbacks{
		OnFavorite: a.handleFavorite,
		OnRed:      func() { a.handleColorToggle(model.ColorRed) },
		OnGreen:    func() { a.handleColorToggle(model.ColorGreen) },
		OnBlue:     func() { a.handleColorToggle(model.ColorBlue) },
		OnDelete:   a.handleDelete,
	})

	notifier := ui.NewNotifier()
	a.mainWindow = ui.NewMainWindow(fyneApp, a.actionPanel, a.fileBrowser, a.viewer, notifier)

	ui.SetupShortcuts(a.mainWindow.Window().Canvas(), ui.ShortcutCallbacks{
		OnRed:      func() { a.handleColorToggle(model.ColorRed) },
		OnGreen:    func() { a.handleColorToggle(model.ColorGreen) },
		OnBlue:     func() { a.handleColorToggle(model.ColorBlue) },
		OnDelete:   a.handleDelete,
		OnNext:     a.handleNext,
		OnPrevious: a.handlePrevious,
	})

	a.loadInitialDirectory()
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
	img, err := service.LoadOrientedImage(photo.ImagePath)
	if err != nil {
		a.viewer.Clear()
		a.showError("Failed to load image", err)
		return
	}
	a.viewer.ShowPhoto(img)
	a.updateColorIndicators(photo)
	a.actionPanel.SetFavoriteEnabled(photo.IsJPEG())
	a.favoriteMenuItem.Disabled = !photo.IsJPEG()
}

func (a *Application) updateColorIndicators(photo model.Photo) {
	colors, err := a.colorService.GetColors(photo)
	if err != nil {
		a.showError("Failed to get colors", err)
		return
	}
	a.viewer.SetColorIndicators(colors)
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
}

func (a *Application) handleSecondaryTap(pos fyne.Position) {
	ui.ShowContextMenu(a.contextMenu, a.mainWindow.Window().Canvas(), pos)
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
