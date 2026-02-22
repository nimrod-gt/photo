package app

import (
	"log"
	"os"

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

	toolbar     *ui.Toolbar
	fileBrowser *ui.FileBrowser
	viewer      *ui.Viewer
	mainWindow  *ui.MainWindow
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
	fyneApp := fyneapp.New()
	fyneApp.Settings().SetTheme(ui.NewDarkTheme())

	a.toolbar = ui.NewToolbar(ui.ToolbarCallbacks{
		OnFavorite: a.handleFavorite,
		OnRed:      func() { a.handleColorToggle(model.ColorRed) },
		OnGreen:    func() { a.handleColorToggle(model.ColorGreen) },
		OnBlue:     func() { a.handleColorToggle(model.ColorBlue) },
		OnDelete:   a.handleDelete,
	})

	a.fileBrowser = ui.NewFileBrowser(ui.FileBrowserCallbacks{
		OnPhotoSelected: a.handlePhotoSelected,
	})

	a.viewer = ui.NewViewer(ui.ViewerCallbacks{
		OnTapped: a.handleNext,
	})

	a.mainWindow = ui.NewMainWindow(fyneApp, a.toolbar, a.fileBrowser, a.viewer)

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

func (a *Application) loadInitialDirectory() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Failed to get home directory: %v", err)
		return
	}
	a.loadDirectory(home)
}

func (a *Application) loadDirectory(dir string) {
	photos, err := a.scanner.ScanDirectory(dir)
	if err != nil {
		log.Printf("Failed to scan directory: %v", err)
		return
	}

	a.scanner.SortPhotos(photos, service.SortByName)
	a.navigator.SetPhotos(photos)
	a.fileBrowser.SetPhotos(photos)

	if photo, ok := a.navigator.Current(); ok {
		a.showPhoto(photo)
	}
}

func (a *Application) showPhoto(photo model.Photo) {
	a.viewer.ShowPhoto(photo)
	a.updateColorIndicators(photo)
}

func (a *Application) updateColorIndicators(photo model.Photo) {
	colors, err := a.colorService.GetColors(photo)
	if err != nil {
		log.Printf("Failed to get colors: %v", err)
		return
	}
	a.viewer.SetColorIndicators(colors)
}

func (a *Application) handlePhotoSelected(photo model.Photo) {
	idx := a.navigator.FindIndex(photo.JPEGPath)
	if idx < 0 {
		return
	}
	if p, ok := a.navigator.GoTo(idx); ok {
		a.showPhoto(p)
	}
}

func (a *Application) handleNext() {
	if photo, ok := a.navigator.Next(); ok {
		a.showPhoto(photo)
		a.fileBrowser.SelectIndex(a.navigator.CurrentIndex())
	}
}

func (a *Application) handlePrevious() {
	if photo, ok := a.navigator.Previous(); ok {
		a.showPhoto(photo)
		a.fileBrowser.SelectIndex(a.navigator.CurrentIndex())
	}
}

func (a *Application) handleFavorite() {
	photo, ok := a.navigator.Current()
	if !ok {
		return
	}
	if err := a.exifService.ToggleFavorite(photo.JPEGPath); err != nil {
		log.Printf("Failed to toggle favorite: %v", err)
	}
}

func (a *Application) handleColorToggle(color model.ColorLabel) {
	photo, ok := a.navigator.Current()
	if !ok {
		return
	}
	if err := a.colorService.ToggleColor(photo, color); err != nil {
		log.Printf("Failed to toggle color: %v", err)
	}
	a.updateColorIndicators(photo)
}

func (a *Application) handleDelete() {
	photo, ok := a.navigator.Current()
	if !ok {
		return
	}

	dialog.ShowConfirm("Delete Photo",
		"Delete "+photo.Name+"? This will also delete the RAW pair.",
		func(confirmed bool) {
			if !confirmed {
				return
			}
			if err := a.deleter.Delete(photo); err != nil {
				log.Printf("Failed to delete photo: %v", err)
				return
			}
			if next, ok := a.navigator.RemoveCurrent(); ok {
				a.showPhoto(next)
			} else {
				a.viewer.Clear()
			}
			a.fileBrowser.SetPhotos(a.getNavigatorPhotos())
			a.fileBrowser.SelectIndex(a.navigator.CurrentIndex())
		},
		a.mainWindow.Window(),
	)
}

func (a *Application) getNavigatorPhotos() []model.Photo {
	return a.navigator.Photos()
}
