package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
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
