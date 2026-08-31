package app

import (
	"context"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"photo/internal/core/clipboard"
	"photo/internal/core/library"
	"photo/internal/core/model"
	"photo/internal/gui/ui"
)

func (a *Application) handleColorToggle(color model.ColorLabel) {
	a.toggleCurrentPhoto("Failed to toggle color", false, func(photo model.Photo) (bool, error) {
		return false, a.colorService.ToggleColor(photo, color)
	})
}

func (a *Application) handleFavorite() {
	a.toggleCurrentPhoto("Failed to toggle favorite", true, func(photo model.Photo) (bool, error) {
		return a.exifService.ToggleFavorite(photo.ImagePath)
	})
}

// A rating toggle writes into the photo's XMP packet, so it only runs where the
// scan found one; a colour toggle touches nothing but our own JSON file.
func (a *Application) toggleCurrentPhoto(failure string, ratingWritten bool, toggle func(model.Photo) (bool, error)) {
	if a.shortcutsBlocked() {
		return
	}
	photo, ok := a.navigator.Current()
	if !ok {
		return
	}
	if ratingWritten && !a.metaOf(photo).Ratable {
		return
	}
	go func() {
		favorite, err := toggle(photo)
		if err != nil {
			a.showErrorAsync(failure, err)
			return
		}
		fyne.Do(func() {
			a.refreshChangedPhoto(photo, ratingWritten, favorite)
		})
	}()
}

// The file work of a copy and of a delete, single photo and bulk alike, goes
// through these two: a photo is only touched once no run is going to write for
// it any more. Both wait, so both belong on a worker goroutine.
func (a *Application) copyPhotoFiles(ctx context.Context, photo model.Photo, dest string, mode library.CopyMode) error {
	// A run writes the sidecar of the photo, and only a copy that takes the RAW
	// takes the sidecar with it, so every other copy has nothing to wait for.
	if photo.HasRAW() && mode != library.CopyJPEGOnly {
		a.awaitTags(ctx, photo)
	}
	return library.CopyWithContext(ctx, photo, dest, mode)
}

func (a *Application) deletePhotoFiles(photo model.Photo, includeRAW bool) error {
	a.stopTags(photo)
	if err := library.DeleteWithOption(photo, includeRAW); err != nil {
		return err
	}
	a.imageProvider.Forget(photo.ImagePath)
	return nil
}

func (a *Application) handleCopyToClipboard() {
	if a.shortcutsBlocked() {
		return
	}
	photo, ok := a.navigator.Current()
	if !ok {
		return
	}
	if err := clipboard.CopyImage(photo.ImagePath); err != nil {
		a.showError("Failed to copy to clipboard", err)
		return
	}
	a.notifier.ShowNotification("Copied to clipboard")
}

func (a *Application) handleDelete() {
	if a.dialogBlocked(dialogDelete) {
		return
	}
	photo, ok := a.navigator.Current()
	if !ok {
		return
	}

	message := "Delete " + photo.Name + "?"
	if photo.HasRAW() {
		message += " This will also delete the RAW pair."
	}
	a.showConfirm(dialogDelete, "Delete Photo", "Delete (D)", "Cancel (N)",
		widget.NewLabel(message),
		func() {
			go func() {
				if err := a.deletePhotoFiles(photo, true); err != nil {
					a.showErrorAsync("Failed to delete photo", err)
					return
				}
				if err := a.colorService.RemoveColors(photo); err != nil {
					log.Println("Failed to remove color labels:", err)
				}
				fyne.Do(func() {
					nextPhoto, navIdx, hasNext := a.navigator.RemoveCurrent()
					a.fileBrowser.RemovePhoto(photo.ImagePath)
					if hasNext {
						a.showPhoto(nextPhoto)
						a.fileBrowser.SelectIndex(navIdx)
					} else {
						a.clearViewer()
					}
				})
			}()
		})
}

func (a *Application) handleHelp() {
	a.toggleDialog(dialogHelp, func() toggleableDialog {
		return ui.NewHelp(a.mainWindow.Window())
	})
}

// A canvas shortcut fires whenever no widget holds focus, which includes an open
// dialog that has nothing focused inside it, so every photo action goes through
// this guard.
func (a *Application) shortcutsBlocked() bool {
	return a.gridMode || a.dialogs.anyOpen()
}

// The guard is the whole of what the app adds to an action the viewer already
// offers, so it is put on at the wiring instead of in a handler per key.
func (a *Application) guarded(action func()) func() {
	return func() {
		if !a.shortcutsBlocked() {
			action()
		}
	}
}

// A Fyne file picker or popup menu stacks its own overlay on top of ours and
// handles no keys itself, so Escape would otherwise cancel the dialog beneath it.
func (a *Application) foreignOverlayOnTop() bool {
	return len(a.mainWindow.Window().Canvas().Overlays().List()) > 1
}

func (a *Application) handleCancel() {
	if a.foreignOverlayOnTop() {
		return
	}
	a.dialogs.cancel()
}

func (a *Application) handleCopy() {
	if a.dialogBlocked(dialogCopy) {
		return
	}
	photo, ok := a.navigator.Current()
	if !ok {
		return
	}

	destDir, copyMode := a.copyPreferences()
	destEntry := ui.NewDestinationEntry(destDir, a.mainWindow.Window())
	modeSelect := ui.NewCopyModeSelect(copyMode)

	content := ui.NewCopyDialogContent(photo.Name, destEntry.Container, modeSelect)

	a.showConfirm(dialogCopy, "Copy Photo", "Copy (C)", "Cancel (N)",
		content,
		func() {
			dest := destEntry.Text()
			if len(dest) == 0 {
				a.notifier.ShowError("No destination folder selected")
				return
			}
			mode := modeSelect.Mode
			a.saveCopyPreferences(dest, mode)
			go func() {
				err := a.copyPhotoFiles(context.Background(), photo, dest, mode)
				fyne.Do(func() {
					if err != nil {
						a.showError("Failed to copy photo", err)
						return
					}
					if mode == library.CopyWithRAW && !photo.HasRAW() {
						a.notifier.ShowWarning(photo.Name + " copied without RAW (RAW file not found)")
					} else {
						a.notifier.ShowNotification(photo.Name + " copied")
					}
				})
			}()
		})
}
