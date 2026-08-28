package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSettingsDialog(t *testing.T, opts SettingsOptions, callbacks SettingsCallbacks) *SettingsDialog {
	t.Helper()
	app := test.NewTempApp(t)
	window := app.NewWindow("settings")
	t.Cleanup(window.Close)
	return NewSettingsDialog(opts, window, callbacks)
}

func TestSettingsDialogAppliesEveryBoxAtOnce(t *testing.T) {
	applied := map[string]bool{}
	d := newTestSettingsDialog(t, SettingsOptions{}, SettingsCallbacks{
		OnShowTags:       func(on bool) { applied["tags"] = on },
		OnAutoSaveXMP:    func(on bool) { applied["xmp"] = on },
		OnAutoSaveJPEG:   func(on bool) { applied["jpeg"] = on },
		OnShowSaveButton: func(on bool) { applied["save"] = on },
	})

	d.showTags.SetChecked(true)
	d.showSave.SetChecked(true)
	d.autoXMP.SetChecked(true)
	d.autoJPEG.SetChecked(true)

	assert.Equal(t, map[string]bool{"tags": true, "save": true, "xmp": true, "jpeg": true}, applied)
}

// A box filled in from the settings the app already holds is showing them, not
// changing them.
func TestSettingsDialogAppliesNothingWhileItIsBuilt(t *testing.T) {
	applied := 0
	count := func(bool) { applied++ }
	newTestSettingsDialog(t, SettingsOptions{
		ShowTags:       true,
		AutoSaveXMP:    true,
		AutoSaveJPEG:   true,
		ShowSaveButton: true,
	}, SettingsCallbacks{
		OnShowTags:       count,
		OnAutoSaveXMP:    count,
		OnAutoSaveJPEG:   count,
		OnShowSaveButton: count,
	})

	assert.Zero(t, applied)
}

func TestSettingsDialogSaveButtonBox(t *testing.T) {
	d := newTestSettingsDialog(t, SettingsOptions{ShowSaveButton: true}, SettingsCallbacks{})
	require.False(t, d.showSave.Disabled())
	require.True(t, d.showSave.Checked)

	d.autoXMP.SetChecked(true)
	assert.False(t, d.showSave.Disabled(), "one autosave still leaves the button something to do")

	d.autoJPEG.SetChecked(true)
	assert.True(t, d.showSave.Disabled())
	assert.False(t, d.showSave.Checked)

	d.autoJPEG.SetChecked(false)
	assert.False(t, d.showSave.Disabled())
	assert.True(t, d.showSave.Checked, "the choice comes back as the user left it")
}

func TestSettingsDialogEscapeIsHandledByTheDialog(t *testing.T) {
	escaped := 0
	d := newTestSettingsDialog(t, SettingsOptions{}, SettingsCallbacks{
		OnEscape: func() { escaped++ },
	})

	d.showTags.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	assert.Equal(t, 1, escaped)

	d.showTags.TypedRune(' ')
	assert.Equal(t, 1, escaped)
	assert.True(t, d.showTags.Checked, "Space still ticks the box it is pressed over")
}
