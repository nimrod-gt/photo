package app

import (
	"context"
	"fmt"
	"log"
	"slices"

	"fyne.io/fyne/v2"

	"photo/internal/core/library"
	"photo/internal/core/model"
	"photo/internal/gui/ui"
)

func pathSet(photos []model.Photo) map[string]bool {
	paths := make(map[string]bool, len(photos))
	for _, p := range photos {
		paths[p.ImagePath] = true
	}
	return paths
}

func (a *Application) handleDeleteAll() {
	if a.dialogBlocked(dialogDeleteAll) {
		return
	}
	filtered := a.fileBrowser.FilteredPhotos()
	if len(filtered) == 0 {
		return
	}

	content, rawCheck := ui.NewDeleteAllDialogContent(len(filtered))

	a.showConfirm(dialogDeleteAll, "Delete All", "Delete", "Cancel",
		content,
		func() {
			includeRAW := rawCheck.Checked
			go a.runBulkDelete(filtered, includeRAW)
		})
}

func (a *Application) runBulkDelete(photos []model.Photo, includeRAW bool) {
	skipped := 0
	var deleted []model.Photo
	for _, photo := range photos {
		if err := a.deletePhotoFiles(photo, includeRAW); err != nil {
			log.Printf("Failed to delete %s: %v", photo.Name, err)
			skipped++
			continue
		}
		deleted = append(deleted, photo)
	}
	colorsErr := a.colorService.RemoveMultipleColors(deleted)
	fyne.Do(func() {
		if colorsErr != nil {
			a.showError("Failed to remove color labels", colorsErr)
		}
		a.fileBrowser.RemovePhotos(pathSet(deleted))
		a.navigator.SetPhotos(a.fileBrowser.FilteredPhotos())
		if p, navIdx, ok := a.navigator.GoTo(0); ok {
			a.showPhoto(p)
			a.fileBrowser.SelectIndex(navIdx)
		} else {
			a.clearViewer()
		}
		if skipped > 0 {
			a.mainWindow.ShowWarning(fmt.Sprintf("Deleted %d photos (%d failed)", len(deleted), skipped))
		} else {
			a.mainWindow.ShowNotification(fmt.Sprintf("Deleted %d photos", len(deleted)))
		}
	})
}

func (a *Application) handleUnselectAll() {
	if a.dialogBlocked(dialogUnselectAll) {
		return
	}
	activeColors := a.fileBrowser.ActiveFilterColors()
	if len(activeColors) == 0 {
		return
	}

	filtered := a.fileBrowser.FilteredPhotos()
	meta := a.fileBrowser.FilteredMeta()
	var affected []model.Photo
	for i, photo := range filtered {
		if i < len(meta) && slices.ContainsFunc(meta[i].Colors, func(c model.ColorLabel) bool {
			return slices.Contains(activeColors, c)
		}) {
			affected = append(affected, photo)
		}
	}
	if len(affected) == 0 {
		return
	}

	a.showConfirm(dialogUnselectAll, "Unselect All", "Remove", "Cancel",
		ui.NewUnselectAllDialogContent(len(affected), activeColors),
		func() {
			go func() {
				err := a.colorService.RemoveColorLabels(affected, activeColors)
				fyne.Do(func() {
					if err != nil {
						a.showError("Failed to remove color labels", err)
						return
					}
					a.fileBrowser.RemoveColorLabels(pathSet(affected), activeColors)
					for _, c := range activeColors {
						a.fileBrowser.ToggleColorFilter(c)
					}
					a.fileBrowser.ClearPinnedPath()
					a.reapplyFilter()
					a.mainWindow.ShowNotification(fmt.Sprintf("Removed color labels from %d photos", len(affected)))
				})
			}()
		})
}

func (a *Application) handleCopyAll() {
	if a.dialogBlocked(dialogCopyAll) {
		return
	}
	filtered := a.fileBrowser.FilteredPhotos()
	if len(filtered) == 0 {
		return
	}

	destDir, copyMode := a.copyPreferences()
	ctx, cancel := context.WithCancel(context.Background())

	var copyAllDialog *ui.CopyAllDialog
	closeDialog := func() {
		if a.dialogs.isCurrent(copyAllDialog) {
			a.dialogs.closed()
		}
		copyAllDialog.Hide()
	}
	// Once the copy runs, the dialog stays open and registered until the
	// goroutine stops: cancelling only signals the context, and the goroutine
	// closes the dialog itself. Closing it early would let a second Copy All
	// open while the first one still copies.
	cancelCopy := func() {
		cancel()
		if copyAllDialog.Copying() {
			copyAllDialog.Cancelling()
			return
		}
		closeDialog()
	}
	copyAllDialog = ui.NewCopyAllDialog(len(filtered), destDir, copyMode, a.mainWindow.Window(),
		func() { a.beginBulkCopy(ctx, cancel, filtered, copyAllDialog, closeDialog) },
		cancelCopy,
	)
	a.dialogs.openSelfClosing(dialogCopyAll, copyAllDialog, cancelCopy)
	copyAllDialog.Show()
}

func (a *Application) beginBulkCopy(
	ctx context.Context,
	cancel context.CancelFunc,
	photos []model.Photo,
	copyAllDialog *ui.CopyAllDialog,
	closeDialog func(),
) {
	dest := copyAllDialog.DestDir()
	if len(dest) == 0 {
		cancel()
		a.mainWindow.ShowError("No destination folder selected")
		closeDialog()
		return
	}
	mode := copyAllDialog.CopyMode()
	a.saveCopyPreferences(dest, mode)
	copyAllDialog.CopyStarted()

	go func() {
		defer cancel()
		a.runBulkCopy(ctx, photos, dest, mode, copyAllDialog, closeDialog)
	}()
}

func (a *Application) runBulkCopy(
	ctx context.Context,
	photos []model.Photo,
	dest string,
	mode library.CopyMode,
	copyAllDialog *ui.CopyAllDialog,
	closeDialog func(),
) {
	total := len(photos)
	copied := 0
	skipped := 0
	for i, photo := range photos {
		if err := a.copyPhotoFiles(ctx, photo, dest, mode); err != nil {
			if ctx.Err() != nil {
				fyne.Do(func() {
					a.mainWindow.ShowWarning(fmt.Sprintf("Copy cancelled after %d/%d photos", copied, total))
					closeDialog()
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
			copyAllDialog.SetProgress(progress)
		})
	}
	// read before the deferred cancel can taint it: the fyne.Do closure runs
	// after this goroutine may already be gone
	cancelled := ctx.Err() != nil
	fyne.Do(func() {
		closeDialog()
		if cancelled {
			a.mainWindow.ShowWarning(fmt.Sprintf("Copy cancelled after %d/%d photos", copied, total))
			return
		}
		if skipped > 0 {
			a.mainWindow.ShowWarning(fmt.Sprintf("Copied %d/%d photos (%d skipped)", copied, total, skipped))
		} else {
			a.mainWindow.ShowNotification(fmt.Sprintf("Copied %d/%d photos", copied, total))
		}
	})
}
