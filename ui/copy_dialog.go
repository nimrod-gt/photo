package ui

import (
	"photo/service"

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

var copyModeLabels = []string{"Photo", "Photo + RAW", "RAW"}

type CopyModeSelect struct {
	radio *widget.RadioGroup
	Mode  service.CopyMode
}

func NewCopyModeSelect(initial service.CopyMode) *CopyModeSelect {
	cms := &CopyModeSelect{Mode: initial}
	cms.radio = widget.NewRadioGroup(copyModeLabels, func(selected string) {
		switch selected {
		case copyModeLabels[0]:
			cms.Mode = service.CopyJPEGOnly
		case copyModeLabels[1]:
			cms.Mode = service.CopyWithRAW
		case copyModeLabels[2]:
			cms.Mode = service.CopyOnlyRAW
		}
	})
	cms.radio.Horizontal = true
	cms.radio.SetSelected(copyModeLabels[initial])
	return cms
}

const copyDialogWidth = float32(500)

func NewCopyDialogContent(filename string, destRow *fyne.Container, modeSelect *CopyModeSelect) *fyne.Container {
	nameLabel := widget.NewLabel(filename)
	nameLabel.TextStyle = fyne.TextStyle{Bold: true}
	content := container.NewVBox(
		nameLabel,
		destRow,
		modeSelect.radio,
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
