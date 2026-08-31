package app

import (
	"photo/internal/gui/ui"
)

// The window is opened by the key that once toggled the tag overlay, and the
// overlay is a box inside it now: every setting is applied and remembered the
// moment its box is ticked, so the window has nothing to confirm on the way out.
func (a *Application) handleSettings() {
	a.toggleDialog(dialogSettings, func() toggleableDialog {
		return ui.NewSettingsDialog(ui.SettingsOptions{
			ShowTags:       a.showTags,
			AutoSaveXMP:    a.autoSaveXMP,
			AutoSaveJPEG:   a.autoSaveJPEG,
			ShowSaveButton: a.showSaveButton,
		}, a.mainWindow.Window(), ui.SettingsCallbacks{
			OnShowTags:       a.setTagsVisible,
			OnAutoSaveXMP:    a.boolSetting(&a.autoSaveXMP, autoSaveXMPKey),
			OnAutoSaveJPEG:   a.boolSetting(&a.autoSaveJPEG, autoSaveJPEGKey),
			OnShowSaveButton: a.boolSetting(&a.showSaveButton, showSaveButtonKey),
			OnEscape:         a.handleCancel,
		})
	})
}
