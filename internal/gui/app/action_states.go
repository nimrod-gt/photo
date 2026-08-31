package app

import (
	"photo/internal/core/model"
	"photo/internal/gui/ui"
)

func (a *Application) updateActionStates(photo model.Photo) {
	colors, err := a.colorService.GetColors(photo)
	if err != nil {
		// Unreadable colours are still a state to show: leaving the buttons as
		// they were would have them describe the photo before this one.
		a.showError("Failed to get colors", err)
	}
	a.updateColorButtonStates(colors)
	a.updateFavoriteButtonStates(photo)
}

// Nothing is on screen, so the buttons describe nothing: leaving them as they
// were would let the next keystroke act on a photo the user is no longer
// looking at.
func (a *Application) clearActionStates() {
	a.updateColorButtonStates(nil)
	a.setFavoriteButtonStates(false, false)
}

func (a *Application) clearViewer() {
	a.viewer.Clear()
	a.clearActionStates()
	a.clearTags()
}

func (a *Application) metaOf(photo model.Photo) model.PhotoMeta {
	idx := a.navigator.FindIndex(photo.ImagePath)
	if idx < 0 {
		return model.PhotoMeta{}
	}
	return a.fileBrowser.GetMeta(idx)
}

// The rating is read once per folder, with the thumbnails, and kept in the
// browser's meta; the file is not opened again for the viewer.
func (a *Application) updateFavoriteButtonStates(photo model.Photo) {
	meta := a.metaOf(photo)
	a.setFavoriteButtonStates(meta.Favorite, meta.Ratable)
}

// The button is enabled only where a press can succeed: the scan has looked
// for a packet the rating can be written into, and a JPEG without one would
// answer every press with the same error.
func (a *Application) setFavoriteButtonStates(favorite, ratable bool) {
	a.actionPanel.SetFavoriteEnabled(ratable)
	a.actionPanel.SetFavoriteActive(favorite)
	a.contextMenuItems.Favorite.Disabled = !ratable
	a.contextMenuItems.Favorite.Checked = favorite
}

func (a *Application) updateColorButtonStates(colors []model.ColorLabel) {
	activeSet := ui.ColorSet(colors)
	for _, label := range ui.ColorOrder {
		a.actionPanel.SetColorActive(label, activeSet[label])
		if item, ok := a.contextMenuItems.Colors[label]; ok {
			item.Checked = activeSet[label]
		}
	}
}

// The ratings arrive with the thumbnails, after the folder is shown, and the
// photo on screen may be one of them. Only the favorite comes from the scan;
// the colours were read when the photo was shown and are not read again here.
func (a *Application) handleMetaLoaded(displayIndex int) {
	if a.gridMode {
		return
	}
	photo, ok := a.navigator.Current()
	if !ok || a.navigator.CurrentIndex() != displayIndex {
		return
	}
	a.updateFavoriteButtonStates(photo)
}

// The toggle runs off the main goroutine, and the photo it changed may no
// longer be the one on screen by the time it lands: the file list is refreshed
// wherever the photo sits, the buttons only while they still describe it. The
// rating is written into the list only when the app is the one that wrote it
// into the file, so a colour change does not put a stale one back.
func (a *Application) refreshChangedPhoto(photo model.Photo, ratingWritten, favorite bool) {
	idx := a.navigator.FindIndex(photo.ImagePath)
	colors, err := a.colorService.GetColors(photo)
	if err != nil {
		// The change is already on disk, so the colours the list last read stand
		// in for the ones that could not be read now: dropping the refresh would
		// leave the star and the buttons describing the state before it.
		if idx >= 0 {
			colors = a.fileBrowser.GetMeta(idx).Colors
		}
		a.showError("Failed to get colors", err)
	}
	if idx >= 0 {
		if ratingWritten {
			a.fileBrowser.RefreshItemMeta(idx, colors, favorite)
		} else {
			a.fileBrowser.RefreshItemColors(idx, colors)
		}
	}
	if current, ok := a.navigator.Current(); ok && current.ImagePath == photo.ImagePath {
		a.updateColorButtonStates(colors)
		a.updateFavoriteButtonStates(photo)
	}
	if a.fileBrowser.HasFilter() {
		a.fileBrowser.SetPinnedPath(photo.ImagePath)
		a.reapplyFilter()
	}
}
