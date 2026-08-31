package ui

import (
	"slices"

	"photo/internal/core/library"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type DestinationEntry struct {
	entry     *widget.Entry
	browseBtn *widget.Button
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
		browseBtn: browseBtn,
		Container: container.NewBorder(nil, nil, nil, browseBtn, entry),
	}
}

func (d *DestinationEntry) Text() string {
	return d.entry.Text
}

func (d *DestinationEntry) Disable() {
	d.entry.Disable()
	d.browseBtn.Disable()
}

var copyModeLabels = []string{"Photo", "Photo + RAW", "RAW"}

type CopyModeSelect struct {
	radio *widget.RadioGroup
	Mode  library.CopyMode
}

func NewCopyModeSelect(initial library.CopyMode) *CopyModeSelect {
	if int(initial) < 0 || int(initial) >= len(copyModeLabels) {
		initial = library.CopyWithRAW
	}
	cms := &CopyModeSelect{Mode: initial}
	// The labels stand in the order of the modes they name, so the index is the
	// mode; a selection the table does not know leaves the mode where it was.
	cms.radio = widget.NewRadioGroup(copyModeLabels, func(selected string) {
		if i := slices.Index(copyModeLabels, selected); i >= 0 {
			cms.Mode = library.CopyMode(i)
		}
	})
	cms.radio.Horizontal = true
	cms.radio.SetSelected(copyModeLabels[initial])
	return cms
}

const copyDialogWidth = float32(500)

func NewCopyDialogContent(filename string, destRow *fyne.Container, modeSelect *CopyModeSelect) *fyne.Container {
	nameLabel := boldLabel(filename)
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
