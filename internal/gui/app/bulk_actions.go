package app

import (
	"context"
	"fmt"
	"log"
	"slices"

	"fyne.io/fyne/v2"

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
			go func() {
				deleted := 0
				skipped := 0
				var deletedPhotos []model.Photo
				for _, photo := range filtered {
					if err := a.deleter.DeleteWithOption(photo, includeRAW); err != nil {
						log.Printf("Failed to delete %s: %v", photo.Name, err)
						skipped++
						continue
					}
					deleted++
					deletedPhotos = append(deletedPhotos, photo)
				}
				colorsErr := a.colorService.RemoveMultipleColors(deletedPhotos)
				for _, photo := range deletedPhotos {
					a.imageProvider.Forget(photo.ImagePath)
				}
				fyne.Do(func() {
					if colorsErr != nil {
						a.showError("Failed to remove color labels", colorsErr)
					}
					a.fileBrowser.RemovePhotos(pathSet(deletedPhotos))
					newFiltered := a.fileBrowser.FilteredPhotos()
					a.navigator.SetPhotos(newFiltered)
					if p, navIdx, ok := a.navigator.GoTo(0); ok {
						a.showPhoto(p)
						a.fileBrowser.SelectIndex(navIdx)
					} else {
						a.clearViewer()
					}
					if skipped > 0 {
						a.mainWindow.ShowWarning(fmt.Sprintf("Deleted %d photos (%d failed)", deleted, skipped))
					} else {
						a.mainWindow.ShowNotification(fmt.Sprintf("Deleted %d photos", deleted))
					}
				})
			}()
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
	copying := false

	var copyAllDialog *ui.CopyAllDialog
	closeDialog := func() {
		a.dialogs.closed()
		copyAllDialog.Hide()
	}
	// Once the copy runs, the dialog stays open and registered until the
	// goroutine stops: cancelling only signals the context, and the goroutine
	// closes the dialog itself. Closing it early would let a second Copy All
	// open while the first one still copies.
	copyAllDialog = ui.NewCopyAllDialog(len(filtered), destDir, copyMode, a.mainWindow.Window(),
		func() {
			dest := copyAllDialog.DestDir()
			if len(dest) == 0 {
				a.mainWindow.ShowError("No destination folder selected")
				closeDialog()
				return
			}
			mode := copyAllDialog.CopyMode()
			a.saveCopyPreferences(dest, mode)
			copying = true

			go func() {
				total := len(filtered)
				copied := 0
				skipped := 0
				for i, photo := range filtered {
					if err := a.copier.CopyWithContext(ctx, photo, dest, mode); err != nil {
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
				fyne.Do(func() {
					closeDialog()
					if skipped > 0 {
						a.mainWindow.ShowWarning(fmt.Sprintf("Copied %d/%d photos (%d skipped)", copied, total, skipped))
					} else {
						a.mainWindow.ShowNotification(fmt.Sprintf("Copied %d/%d photos", copied, total))
					}
				})
			}()
		},
		func() {
			cancel()
			if copying {
				copyAllDialog.Cancelling()
				return
			}
			closeDialog()
		},
	)
	a.dialogs.open(dialogCopyAll, copyAllDialog, nil)
	copyAllDialog.Show()
}
