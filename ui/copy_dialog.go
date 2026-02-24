package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type DestinationEntry struct {
	entry     *widget.Entry
	Container *fyne.Container
}

func NewDestinationEntry(initialPath string, window fyne.Window) *DestinationEntry {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("Select folder...")
	entry.SetText(initialPath)

	browseBtn := widget.NewButton("Browse...", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			entry.SetText(uri.Path())
		}, window)
	})

	return &DestinationEntry{
		entry:     entry,
		Container: container.NewBorder(nil, nil, nil, browseBtn, entry),
	}
}

func (d *DestinationEntry) Text() string {
	return d.entry.Text
}

type RawCheck struct {
	check   *widget.Check
	Checked bool
}

func NewRawCheck(initial bool) *RawCheck {
	rc := &RawCheck{Checked: initial}
	rc.check = widget.NewCheck("Copy with RAW", func(checked bool) {
		rc.Checked = checked
	})
	rc.check.SetChecked(initial)
	return rc
}

const copyDialogWidth = float32(500)

func NewCopyDialogContent(filename string, destRow *fyne.Container, rawCheck *RawCheck) *fyne.Container {
	nameLabel := widget.NewLabel(filename)
	nameLabel.TextStyle = fyne.TextStyle{Bold: true}
	content := container.NewVBox(
		nameLabel,
		destRow,
		rawCheck.check,
	)
	return container.New(&minWidthLayout{width: copyDialogWidth}, content)
}

type minWidthLayout struct {
	width float32
}

func (l *minWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	s := objects[0].MinSize()
	if s.Width < l.width {
		s.Width = l.width
	}
	return s
}

func (l *minWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	objects[0].Resize(size)
}
