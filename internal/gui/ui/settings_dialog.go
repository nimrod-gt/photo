package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type SettingsCallbacks struct {
	OnShowTags       func(bool)
	OnAutoSaveXMP    func(bool)
	OnAutoSaveJPEG   func(bool)
	OnShowSaveButton func(bool)
	OnEscape         func()
}

type SettingsOptions struct {
	ShowTags       bool
	AutoSaveXMP    bool
	AutoSaveJPEG   bool
	ShowSaveButton bool
}

type SettingsDialog struct {
	dialog    *dialog.CustomDialog
	callbacks SettingsCallbacks
	showTags  *dialogCheck
	autoXMP   *dialogCheck
	autoJPEG  *dialogCheck
	showSave  *dialogCheck
	// The Save button setting keeps the value the user gave it while both
	// autosaves hide the button: turning one of them off brings the choice back
	// instead of a box that silently cleared itself.
	saveWanted bool
	applying   bool
}

func NewSettingsDialog(opts SettingsOptions, window fyne.Window, callbacks SettingsCallbacks) *SettingsDialog {
	d := &SettingsDialog{callbacks: callbacks, saveWanted: opts.ShowSaveButton}
	d.build(opts, window)
	return d
}

func (d *SettingsDialog) build(opts SettingsOptions, window fyne.Window) {
	d.showTags = newDialogCheck("Show the tag overlay", func(on bool) {
		d.changed(on, d.callbacks.OnShowTags)
	}, d)
	d.autoXMP = newDialogCheck("Save the XMP sidecar automatically", func(on bool) {
		d.changed(on, d.callbacks.OnAutoSaveXMP)
		d.refreshSaveCheck()
	}, d)
	d.autoJPEG = newDialogCheck("Save the JPEG automatically", func(on bool) {
		d.changed(on, d.callbacks.OnAutoSaveJPEG)
		d.refreshSaveCheck()
	}, d)
	d.showSave = newDialogCheck("Show the Save button in the Tags dialog", func(on bool) {
		if d.applying {
			return
		}
		d.saveWanted = on
		d.changed(on, d.callbacks.OnShowSaveButton)
	}, d)

	d.apply(func() {
		d.showTags.SetChecked(opts.ShowTags)
		d.autoXMP.SetChecked(opts.AutoSaveXMP)
		d.autoJPEG.SetChecked(opts.AutoSaveJPEG)
		d.showSave.SetChecked(opts.ShowSaveButton)
	})
	d.refreshSaveCheck()

	content := container.NewVBox(d.showTags, widget.NewSeparator(), d.autoXMP, d.autoJPEG, d.showSave)
	d.dialog = dialog.NewCustom("Settings", "Close", content, window)
}

// A box set from here is showing a decision rather than making one, so the
// setting behind it is left alone. The flag is put back rather than cleared:
// setting one box refreshes another, and the inner refresh would otherwise open
// the boxes left to set to callbacks of their own.
func (d *SettingsDialog) apply(set func()) {
	was := d.applying
	d.applying = true
	set()
	d.applying = was
}

func (d *SettingsDialog) changed(on bool, apply func(bool)) {
	if d.applying {
		return
	}
	if apply != nil {
		apply(on)
	}
}

// With both files saving themselves the button is gone from the Tags dialog, so
// the box that shows it has nothing left to say.
func (d *SettingsDialog) refreshSaveCheck() {
	both := d.autoXMP.Checked && d.autoJPEG.Checked
	d.apply(func() { d.showSave.SetChecked(d.saveWanted && !both) })
	if both {
		d.showSave.Disable()
		return
	}
	d.showSave.Enable()
}

func (d *SettingsDialog) handleKey(ev *fyne.KeyEvent) bool {
	if ev.Name != fyne.KeyEscape {
		return false
	}
	call(d.callbacks.OnEscape)
	return true
}

func (d *SettingsDialog) handleShortcut(fyne.Shortcut) bool {
	return false
}

func (d *SettingsDialog) trackKey(*fyne.KeyEvent, bool) {}

func (d *SettingsDialog) Show() {
	d.dialog.Show()
}

func (d *SettingsDialog) Hide() {
	d.dialog.Hide()
}

func (d *SettingsDialog) SetOnClosed(closed func()) {
	d.dialog.SetOnClosed(closed)
}
