package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"photo/internal/gui/keyname"
)

type shortcutEntry struct {
	key         string
	description string
}

type shortcutSection struct {
	title   string
	entries []shortcutEntry
}

var helpLeft = []shortcutSection{
	{
		title: "Navigation",
		entries: []shortcutEntry{
			{"← ↑", "Previous photo"},
			{"→ ↓ 🖱", "Next photo"},
			{"+ 🖲↑", "Zoom in"},
			{"- 🖲↓", "Zoom out"},
		},
	},
	{
		title: "Actions",
		entries: []shortcutEntry{
			{"F", "Toggle favorite"},
			{"R", "Toggle red label"},
			{"G", "Toggle green label"},
			{"B", "Toggle blue label"},
			{"D", "Delete photo"},
			{"C", "Copy photo"},
			{"Y", "Copy to clipboard"},
			{"L", "Toggle grid view"},
			{"Z", "Reset zoom"},
			{"T", "Generate stock tags"},
			{"I", "Settings"},
			{"H", "Toggle this help"},
		},
	},
}

var helpRight = []shortcutSection{
	{
		title: "Sort & Filter",
		entries: []shortcutEntry{
			{"S", "Cycle sort order"},
			{"1", "Filter by favorite"},
			{"2", "Filter by red"},
			{"3", "Filter by green"},
			{"4", "Filter by blue"},
		},
	},
	{
		title: "Tags dialog",
		entries: []shortcutEntry{
			{"Tab", "Next field or button"},
			{keyname.Shift + "+Tab", "Previous field or button"},
			{"Space/Enter", "Press the focused button"},
			{keyname.Shift + "+Enter", "Generate"},
			{keyname.Ctrl + "+Enter", "Generate in background"},
			{keyname.Alt + "+C", "Copy the tags"},
			{keyname.Alt + "+V", "Paste the tags"},
			{"Esc", "Stop the generation, or close"},
		},
	},
}

func newKeyLabel(text string) *widget.Label {
	key := widget.NewLabel(text)
	key.TextStyle = fyne.TextStyle{Monospace: true}
	return key
}

// The keys stand in a column of their own, and the widest of them is what that
// column is worth: a fixed width leaves the longer ones running under the
// description beside them.
func keyColumnWidth(sections []shortcutSection) float32 {
	var width float32
	for _, section := range sections {
		for _, entry := range section.entries {
			width = max(width, newKeyLabel(entry.key).MinSize().Width)
		}
	}
	return width
}

func buildHelpColumn(sections []shortcutSection) *fyne.Container {
	col := container.NewVBox()
	keyWidth := keyColumnWidth(sections)
	for i, section := range sections {
		if i > 0 {
			col.Add(widget.NewSeparator())
		}
		title := widget.NewLabel(section.title)
		title.TextStyle = fyne.TextStyle{Bold: true}
		col.Add(title)
		for _, entry := range section.entries {
			row := container.NewHBox(
				container.New(layout.NewGridWrapLayout(fyne.NewSize(keyWidth, 0)), newKeyLabel(entry.key)),
				widget.NewLabel(entry.description),
			)
			col.Add(row)
		}
	}
	return col
}

func NewHelp(window fyne.Window) *dialog.CustomDialog {
	left := buildHelpColumn(helpLeft)
	right := buildHelpColumn(helpRight)
	content := container.NewHBox(left, widget.NewSeparator(), right)
	return dialog.NewCustom("Keyboard Shortcuts", "Close", content, window)
}
