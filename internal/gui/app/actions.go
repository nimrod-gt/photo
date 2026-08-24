package app

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"photo/internal/core/clipboard"
	"photo/internal/core/library"
	"photo/internal/core/model"
	"photo/internal/gui/ui"
)

func (a *Application) handleColorToggle(color model.ColorLabel) {
	if a.shortcutsBlocked() {
		return
	}
	photo, ok := a.navigator.Current()
	if !ok {
		return
	}
	go func() {
		if err := a.colorService.ToggleColor(photo, color); err != nil {
			a.showErrorAsync("Failed to toggle color", err)
			return
		}
		fyne.Do(func() {
			a.photoStateChanged(photo, a.favoriteOf(photo), false)
		})
	}()
}

func (a *Application) handleFavorite() {
	if a.shortcutsBlocked() {
		return
	}
	photo, ok := a.navigator.Current()
	if !ok || !a.metaOf(photo).Ratable {
		return
	}
	go func() {
		favorite, err := a.exifService.ToggleFavorite(photo.ImagePath)
		if err != nil {
			a.showErrorAsync("Failed to toggle favorite", err)
			return
		}
		fyne.Do(func() {
			a.photoStateChanged(photo, favorite, true)
		})
	}()
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
	a.mainWindow.ShowNotification("Copied to clipboard")
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
				if err := a.deleter.Delete(photo); err != nil {
					a.showErrorAsync("Failed to delete photo", err)
					return
				}
				if err := a.colorService.RemoveColors(photo); err != nil {
					log.Println("Failed to remove color labels:", err)
				}
				a.imageProvider.Forget(photo.ImagePath)
				fyne.Do(func() {
					nextPhoto, navIdx, _, hasNext := a.navigator.RemoveCurrent()
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
	if a.dialogs.isOpen(dialogHelp) {
		a.dialogs.cancel()
		return
	}
	if a.dialogs.anyOpen() {
		return
	}
	helpDialog := ui.NewHelp(a.mainWindow.Window())
	helpDialog.SetOnClosed(func() {
		if a.dialogs.isOpen(dialogHelp) {
			a.dialogs.closed()
		}
	})
	a.dialogs.open(dialogHelp, helpDialog, nil)
	helpDialog.Show()
}

// A canvas shortcut fires whenever no widget holds focus, which includes an open
// dialog that has nothing focused inside it, so every photo action goes through
// this guard.
func (a *Application) shortcutsBlocked() bool {
	return a.gridMode || a.dialogs.anyOpen()
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
				a.mainWindow.ShowError("No destination folder selected")
				return
			}
			mode := modeSelect.Mode
			a.saveCopyPreferences(dest, mode)
			go func() {
				err := a.copier.Copy(photo, dest, mode)
				fyne.Do(func() {
					if err != nil {
						a.showError("Failed to copy photo", err)
						return
					}
					if mode == library.CopyWithRAW && !photo.HasRAW() {
						a.mainWindow.ShowWarning(photo.Name + " copied without RAW (RAW file not found)")
					} else {
						a.mainWindow.ShowNotification(photo.Name + " copied")
					}
				})
			}()
		})
}
