package app

import (
	"log"

	"fyne.io/fyne/v2"

	"photo/internal/core/model"
)

// The overlay must never describe a photo the user has moved on from, so every
// path that changes what is on screen bumps the generation, and a read that
// lands afterwards is dropped. The counter is touched on the UI goroutine only,
// which is what makes it enough on its own.
func (a *Application) showTagsFor(photo model.Photo) {
	a.tagGeneration++
	if info, ok := a.imageProvider.PeekStockInfo(photo.ImagePath); ok {
		a.mainWindow.SetPhotoTags(info.Tags)
		return
	}

	a.mainWindow.ClearPhotoTags()
	generation := a.tagGeneration
	go func() {
		info, err := a.imageProvider.StockInfo(photo)
		if err != nil {
			log.Println("Failed to read tags:", err)
			return
		}
		fyne.Do(func() {
			if generation != a.tagGeneration {
				return
			}
			a.mainWindow.SetPhotoTags(info.Tags)
		})
	}()
}

func (a *Application) clearTags() {
	a.tagGeneration++
	a.mainWindow.ClearPhotoTags()
}

// Reached through tagsSession.storeStock, which a save runs on a worker
// goroutine, so the hop is made here rather than at each call site. The bump
// belongs inside the branch: a save for a photo the user has left behind must
// not cancel the read the photo in front of them is still waiting for.
func (a *Application) setTagsIfCurrent(path string, tags model.Tags) {
	fyne.Do(func() {
		if photo, ok := a.navigator.Current(); ok && photo.ImagePath == path {
			a.tagGeneration++
			a.mainWindow.SetPhotoTags(tags)
		}
	})
}

// Not shortcutsBlocked: this is a display preference, not an action on the
// photo, and grid mode is a place the user may well decide from. The overlay is
// suppressed there either way, so the choice simply takes effect on the way out.
func (a *Application) handleToggleTags() {
	if a.dialogs.anyOpen() {
		return
	}
	a.showTags = !a.showTags
	a.mainWindow.SetTagsVisible(a.showTags)
	a.saveTagVisibility()
}
