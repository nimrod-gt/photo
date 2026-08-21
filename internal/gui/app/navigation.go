package app

import (
	"log"

	"fyne.io/fyne/v2"

	"photo/internal/core/model"
	"photo/internal/gui/ui"
)

func (a *Application) showPhoto(photo model.Photo) {
	img, err := a.imageProvider.Get(photo.ImagePath, a.fullImageSize())
	if err != nil {
		a.viewer.Clear()
		a.showError("Failed to load image", err)
		return
	}
	a.viewer.ShowPhoto(img)
	a.updateIndicators(photo)
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
	if a.shortcutsBlocked() {
		return
	}
	if photo, idx, ok := a.navigator.Next(); ok {
		a.showPhoto(photo)
		a.fileBrowser.SelectIndex(idx)
		a.unpinIfMovedAway(photo)
	}
}

func (a *Application) handlePrevious() {
	if a.shortcutsBlocked() {
		return
	}
	if photo, idx, ok := a.navigator.Previous(); ok {
		a.showPhoto(photo)
		a.fileBrowser.SelectIndex(idx)
		a.unpinIfMovedAway(photo)
	}
}

func (a *Application) showCurrentOrFirst() {
	if photo, ok := a.navigator.Current(); ok {
		a.showPhoto(photo)
		a.fileBrowser.SelectIndex(a.navigator.CurrentIndex())
	} else {
		a.viewer.Clear()
	}
}

func (a *Application) handleSecondaryTap(pos fyne.Position) {
	ui.ShowContextMenu(a.contextMenuItems.Menu, a.mainWindow.Window().Canvas(), pos)
}

func (a *Application) handleToggleGrid() {
	if a.dialogs.anyOpen() {
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
	if a.shortcutsBlocked() {
		return
	}
	a.viewer.ResetZoom()
}

func (a *Application) handleZoomIn() {
	if a.shortcutsBlocked() {
		return
	}
	a.viewer.ZoomIn()
}

func (a *Application) handleZoomOut() {
	if a.shortcutsBlocked() {
		return
	}
	a.viewer.ZoomOut()
}
