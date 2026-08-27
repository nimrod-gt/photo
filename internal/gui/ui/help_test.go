package ui

import (
	"runtime"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"

	"photo/internal/gui/keyname"
)

// A key that outgrows its cell is drawn over the description next to it.
func TestHelpKeyColumnFitsEveryKey(t *testing.T) {
	test.NewTempApp(t)

	for _, sections := range [][]shortcutSection{helpLeft, helpRight} {
		width := keyColumnWidth(sections)
		for _, section := range sections {
			for _, entry := range section.entries {
				assert.LessOrEqualf(t, newKeyLabel(entry.key).MinSize().Width, width,
					"%q does not fit the key column", entry.key)
			}
		}
	}
}

// A Mac keyboard carries no key named Ctrl, Shift or Alt, so nothing on screen
// may name one there.
func TestHelpNamesTheModifiersOfThisPlatform(t *testing.T) {
	shown := []string{generateLabel, regenerateLabel, backgroundLabel}
	for _, section := range append(append([]shortcutSection{}, helpLeft...), helpRight...) {
		for _, entry := range section.entries {
			shown = append(shown, entry.key)
		}
	}

	assert.Contains(t, shown, keyname.Shift+"+Enter")
	assert.Contains(t, shown, keyname.Ctrl+"+Enter")
	assert.Contains(t, shown, keyname.Alt+"+C")

	if runtime.GOOS != "darwin" {
		return
	}
	for _, text := range shown {
		for _, ascii := range []string{"Shift", "Ctrl", "Alt"} {
			assert.NotContainsf(t, text, ascii, "%q spells %s out on a Mac", text, ascii)
		}
	}
}
