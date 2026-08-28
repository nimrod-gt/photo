package app

import (
	"photo/internal/gui/ui"
)

// The window is opened by the key that once toggled the tag overlay, and the
// overlay is a box inside it now: every setting is applied and remembered the
// moment its box is ticked, so the window has nothing to confirm on the way out.
func (a *Application) handleSettings() {
	if a.dialogs.isOpen(dialogSettings) {
		a.dialogs.cancel()
		return
	}
	if a.dialogs.anyOpen() {
		return
	}

	settings := ui.NewSettingsDialog(ui.SettingsOptions{
		ShowTags:       a.showTags,
		AutoSaveXMP:    a.autoSaveXMP,
		AutoSaveJPEG:   a.autoSaveJPEG,
		ShowSaveButton: a.showSaveButton,
	}, a.mainWindow.Window(), ui.SettingsCallbacks{
		OnShowTags: a.setTagsVisible,
		OnAutoSaveXMP: func(on bool) {
			a.autoSaveXMP = on
			a.saveAutoSaveXMP()
		},
		OnAutoSaveJPEG: func(on bool) {
			a.autoSaveJPEG = on
			a.saveAutoSaveJPEG()
		},
		OnShowSaveButton: func(on bool) {
			a.showSaveButton = on
			a.saveShowSaveButton()
		},
		OnEscape: a.handleCancel,
	})
	settings.SetOnClosed(func() {
		if a.dialogs.isOpen(dialogSettings) {
			a.dialogs.closed()
		}
	})
	a.dialogs.open(dialogSettings, settings, nil)
	settings.Show()
}
