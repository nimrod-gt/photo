package app

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"photo/internal/core/clipboard"
	"photo/internal/core/imaging"
	"photo/internal/core/library"
	"photo/internal/core/model"
	"photo/internal/core/tags"
	"photo/internal/gui/ui"
)

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
			fyne.Do(func() {
				a.showError("Failed to toggle color", err)
			})
			return
		}
		fyne.Do(func() {
			a.updateColorIndicators(photo)
			a.refreshFileBrowserItem(photo)
			if a.fileBrowser.HasFilter() {
				a.fileBrowser.SetPinnedPath(photo.ImagePath)
				a.reapplyFilter()
			}
		})
	}()
}

func (a *Application) refreshFileBrowserItem(photo model.Photo) {
	idx := a.navigator.FindIndex(photo.ImagePath)
	if idx < 0 {
		return
	}
	colors, err := a.colorService.GetColors(photo)
	if err != nil {
		log.Printf("Failed to get colors for %s: %v", photo.Name, err)
		return
	}
	meta := a.fileBrowser.GetMeta(idx)
	a.fileBrowser.RefreshItemMeta(idx, colors, meta.Favorite)
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
	if a.gridMode {
		return
	}
	if a.dialogs.isOpen(dialogDelete) {
		a.dialogs.confirm()
		return
	}
	if a.dialogs.anyOpen() {
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
	deleteDialog := dialog.NewCustomConfirm("Delete Photo",
		"Delete (D)", "Cancel (N)",
		widget.NewLabel(message),
		func(confirmed bool) {
			a.dialogs.closed()
			if !confirmed {
				return
			}
			go func() {
				if err := a.deleter.Delete(photo); err != nil {
					fyne.Do(func() {
						a.showError("Failed to delete photo", err)
					})
					return
				}
				if err := a.colorService.RemoveColors(photo); err != nil {
					log.Println("Failed to remove color labels:", err)
				}
				fyne.Do(func() {
					nextPhoto, navIdx, _, hasNext := a.navigator.RemoveCurrent()
					a.fileBrowser.RemovePhoto(photo.ImagePath)
					if hasNext {
						a.showPhoto(nextPhoto)
						a.fileBrowser.SelectIndex(navIdx)
					} else {
						a.viewer.Clear()
					}
				})
			}()
		},
		a.mainWindow.Window(),
	)
	a.dialogs.open(dialogDelete, deleteDialog, nil)
	deleteDialog.Show()
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
	if a.gridMode {
		return
	}
	if a.dialogs.isOpen(dialogCopy) {
		a.dialogs.confirm()
		return
	}
	if a.dialogs.anyOpen() {
		return
	}

	photo, ok := a.navigator.Current()
	if !ok {
		return
	}

	prefs := a.fyneApp.Preferences()
	destDir := prefs.String("copyDestination")
	copyMode := library.CopyMode(prefs.IntWithFallback("copyMode", int(library.CopyWithRAW)))

	destEntry := ui.NewDestinationEntry(destDir, a.mainWindow.Window())
	modeSelect := ui.NewCopyModeSelect(copyMode)

	content := ui.NewCopyDialogContent(photo.Name, destEntry.Container, modeSelect)

	copyDialog := dialog.NewCustomConfirm("Copy Photo", "Copy (C)", "Cancel (N)",
		content,
		func(confirmed bool) {
			a.dialogs.closed()
			if !confirmed {
				return
			}
			dest := destEntry.Text()
			if len(dest) == 0 {
				a.mainWindow.ShowError("No destination folder selected")
				return
			}
			mode := modeSelect.Mode
			prefs.SetString("copyDestination", dest)
			prefs.SetInt("copyMode", int(mode))
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
		},
		a.mainWindow.Window(),
	)
	a.dialogs.open(dialogCopy, copyDialog, nil)
	copyDialog.Show()
}

func (a *Application) handleDeleteAll() {
	if a.gridMode || a.dialogs.anyOpen() {
		return
	}
	filtered := a.fileBrowser.FilteredPhotos()
	if len(filtered) == 0 {
		return
	}

	content, rawCheck := ui.NewDeleteAllDialogContent(len(filtered))

	deleteAllDialog := dialog.NewCustomConfirm("Delete All",
		"Delete", "Cancel",
		content,
		func(confirmed bool) {
			a.dialogs.closed()
			if !confirmed {
				return
			}
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
				fyne.Do(func() {
					if colorsErr != nil {
						a.showError("Failed to remove color labels", colorsErr)
					}
					paths := make(map[string]bool, len(deletedPhotos))
					for _, p := range deletedPhotos {
						paths[p.ImagePath] = true
					}
					a.fileBrowser.RemovePhotos(paths)
					newFiltered := a.fileBrowser.FilteredPhotos()
					a.navigator.SetPhotos(newFiltered)
					if p, navIdx, ok := a.navigator.GoTo(0); ok {
						a.showPhoto(p)
						a.fileBrowser.SelectIndex(navIdx)
					} else {
						a.viewer.Clear()
					}
					if skipped > 0 {
						a.mainWindow.ShowWarning(fmt.Sprintf("Deleted %d photos (%d failed)", deleted, skipped))
					} else {
						a.mainWindow.ShowNotification(fmt.Sprintf("Deleted %d photos", deleted))
					}
				})
			}()
		},
		a.mainWindow.Window(),
	)
	a.dialogs.open(dialogDeleteAll, deleteAllDialog, nil)
	deleteAllDialog.Show()
}

func (a *Application) handleUnselectAll() {
	if a.gridMode || a.dialogs.anyOpen() {
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

	unselectDialog := dialog.NewCustomConfirm("Unselect All",
		"Remove", "Cancel",
		ui.NewUnselectAllDialogContent(len(affected), activeColors),
		func(confirmed bool) {
			a.dialogs.closed()
			if !confirmed {
				return
			}
			go func() {
				err := a.colorService.RemoveColorLabels(affected, activeColors)
				fyne.Do(func() {
					if err != nil {
						a.showError("Failed to remove color labels", err)
						return
					}
					paths := make(map[string]bool, len(affected))
					for _, p := range affected {
						paths[p.ImagePath] = true
					}
					a.fileBrowser.RemoveColorLabels(paths, activeColors)
					for _, c := range activeColors {
						a.fileBrowser.ToggleColorFilter(c)
					}
					a.fileBrowser.ClearPinnedPath()
					a.reapplyFilter()
					a.mainWindow.ShowNotification(fmt.Sprintf("Removed color labels from %d photos", len(affected)))
				})
			}()
		},
		a.mainWindow.Window(),
	)
	a.dialogs.open(dialogUnselectAll, unselectDialog, nil)
	unselectDialog.Show()
}

func (a *Application) handleCopyAll() {
	if a.gridMode || a.dialogs.anyOpen() {
		return
	}
	filtered := a.fileBrowser.FilteredPhotos()
	if len(filtered) == 0 {
		return
	}

	prefs := a.fyneApp.Preferences()
	destDir := prefs.String("copyDestination")
	copyMode := library.CopyMode(prefs.IntWithFallback("copyMode", int(library.CopyWithRAW)))

	ctx, cancel := context.WithCancel(context.Background())

	var copyAllDialog *ui.CopyAllDialog
	copyAllDialog = ui.NewCopyAllDialog(len(filtered), destDir, copyMode, a.mainWindow.Window(),
		func() {
			dest := copyAllDialog.DestDir()
			if len(dest) == 0 {
				a.mainWindow.ShowError("No destination folder selected")
				copyAllDialog.Finish()
				return
			}
			mode := copyAllDialog.CopyMode()
			prefs.SetString("copyDestination", dest)
			prefs.SetInt("copyMode", int(mode))

			go func() {
				total := len(filtered)
				copied := 0
				skipped := 0
				for i, photo := range filtered {
					if err := a.copier.CopyWithContext(ctx, photo, dest, mode); err != nil {
						if ctx.Err() != nil {
							fyne.Do(func() {
								a.mainWindow.ShowWarning(fmt.Sprintf("Copy cancelled after %d/%d photos", copied, total))
								if a.dialogs.isOpen(dialogCopyAll) {
									a.dialogs.closed()
									copyAllDialog.Hide()
								}
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
						if a.dialogs.isOpen(dialogCopyAll) {
							copyAllDialog.SetProgress(progress)
						}
					})
				}
				fyne.Do(func() {
					if a.dialogs.isOpen(dialogCopyAll) {
						a.dialogs.closed()
						copyAllDialog.Finish()
					}
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
			if a.dialogs.isOpen(dialogCopyAll) {
				a.dialogs.closed()
			}
			copyAllDialog.Hide()
		},
	)
	a.dialogs.open(dialogCopyAll, copyAllDialog, cancel)
	copyAllDialog.Show()
}

func (a *Application) handleTags() {
	if a.shortcutsBlocked() {
		return
	}
	photo, ok := a.navigator.Current()
	if !ok {
		return
	}

	prefs := a.fyneApp.Preferences()
	ctx, cancel := context.WithCancel(context.Background())

	opts := ui.TagsDialogOptions{
		Filename:   photo.Name,
		ClaudePath: prefs.String("claudePath"),
		Date:       photo.ModTime,
		HasRAW:     photo.HasRAW(),
		IsJPEG:     photo.IsJPEG(),
	}

	var tagsDialog *ui.TagsDialog
	tagsDialog = ui.NewTagsDialog(opts, a.mainWindow.Window(), ui.TagsDialogCallbacks{
		OnGenerate: func() {
			req := tags.Request{Photo: photo, Notes: tagsDialog.Notes(), ClaudePath: tagsDialog.ClaudePath()}
			prefs.SetString("claudePath", req.ClaudePath)
			a.generateTags(ctx, req, tagsDialog)
		},
		OnCopyTitle: func() {
			a.fyneApp.Clipboard().SetContent(tagsDialog.Tags().Title)
			a.mainWindow.ShowNotification("Title copied to clipboard")
		},
		OnCopyKeywords: func() {
			a.fyneApp.Clipboard().SetContent(tagsDialog.Tags().KeywordLine())
			a.mainWindow.ShowNotification("Keywords copied to clipboard")
		},
		OnSaveSidecar: func() {
			sidecarPath := imaging.SidecarPath(photo.RAWPath)
			a.saveTags(tagsDialog.Tags(), filepath.Base(sidecarPath), func(saved model.Tags) error {
				return imaging.WriteSidecar(sidecarPath, saved)
			})
		},
		OnSaveJPEG: func() {
			a.saveTags(tagsDialog.Tags(), photo.Name, func(saved model.Tags) error {
				return a.exifService.WriteStockTags(photo.ImagePath, saved)
			})
		},
		OnClose: func() {
			cancel()
			if a.dialogs.isCurrent(tagsDialog) {
				a.dialogs.closed()
			}
			tagsDialog.Hide()
		},
	})
	a.dialogs.open(dialogTags, tagsDialog, cancel)
	tagsDialog.Show()
	a.prefillTags(photo, tagsDialog)
}

func (a *Application) saveTags(tags model.Tags, target string, save func(model.Tags) error) {
	go func() {
		err := save(tags)
		fyne.Do(func() {
			if err != nil {
				a.showError("Failed to save tags to "+target, err)
				return
			}
			a.mainWindow.ShowNotification("Tags saved to " + target)
		})
	}()
}

func (a *Application) generateTags(ctx context.Context, req tags.Request, tagsDialog *ui.TagsDialog) {
	go func() {
		generated, err := a.tagger.Generate(ctx, req)
		fyne.Do(func() {
			if !a.dialogs.isCurrent(tagsDialog) {
				return
			}
			if err != nil {
				log.Println("Failed to generate tags:", err)
				tagsDialog.Fail(err)
				return
			}
			tagsDialog.SetTags(generated)
		})
	}()
}

func (a *Application) prefillTags(photo model.Photo, tagsDialog *ui.TagsDialog) {
	go func() {
		info, err := a.exifService.GetStockInfo(photo)
		if err != nil {
			log.Printf("Failed to read tags of %s: %v", photo.Name, err)
			return
		}
		fyne.Do(func() {
			if a.dialogs.isCurrent(tagsDialog) {
				tagsDialog.SetPhotoInfo(info.Tags, info.Taken)
			}
		})
	}()
}
