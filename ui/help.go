package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
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
		title: "Actions",
		entries: []shortcutEntry{
			{"F", "Toggle favorite"},
			{"R", "Toggle red label"},
			{"G", "Toggle green label"},
			{"B", "Toggle blue label"},
			{"D", "Delete photo"},
			{"C", "Copy photo"},
			{"H", "Show this help"},
		},
	},
}

var helpRight = []shortcutSection{
	{
		title: "Navigation",
		entries: []shortcutEntry{
			{"← ↑", "Previous photo"},
			{"→ ↓ 🖱", "Next photo"},
		},
	},
	{
		title: "Sort & Filter",
		entries: []shortcutEntry{
			{"S", "Cycle sort order"},
			{"1", "Filter by red"},
			{"2", "Filter by green"},
			{"3", "Filter by blue"},
			{"4", "Filter by favorite"},
		},
	},
}

func buildHelpColumn(sections []shortcutSection) *fyne.Container {
	col := container.NewVBox()
	for i, section := range sections {
		if i > 0 {
			col.Add(widget.NewSeparator())
		}
		title := widget.NewLabel(section.title)
		title.TextStyle = fyne.TextStyle{Bold: true}
		col.Add(title)
		for _, entry := range section.entries {
			key := widget.NewLabel(entry.key)
			key.TextStyle = fyne.TextStyle{Monospace: true}
			desc := widget.NewLabel(entry.description)
			row := container.NewHBox(
				container.New(layout.NewGridWrapLayout(fyne.NewSize(50, 0)), key),
				desc,
			)
			col.Add(row)
		}
	}
	return col
}

func ShowHelp(window fyne.Window) {
	left := buildHelpColumn(helpLeft)
	right := buildHelpColumn(helpRight)
	content := container.NewHBox(left, widget.NewSeparator(), right)
	dialog.ShowCustom("Keyboard Shortcuts", "Close", content, window)
}
