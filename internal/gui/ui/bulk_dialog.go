package ui

import (
	"fmt"
	"strings"

	"photo/internal/core/library"
	"photo/internal/core/model"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func NewDeleteAllDialogContent(count int) (*fyne.Container, *widget.Check) {
	label := widget.NewLabel(fmt.Sprintf("Delete %d filtered photos?", count))
	label.TextStyle = fyne.TextStyle{Bold: true}

	rawCheck := widget.NewCheck("Delete with RAW files", nil)
	rawCheck.SetChecked(true)

	content := container.NewVBox(label, rawCheck)
	return container.New(&minWidthLayout{width: copyDialogWidth}, content), rawCheck
}

func NewUnselectAllDialogContent(count int, colors []model.ColorLabel) *fyne.Container {
	names := make([]string, 0, len(colors))
	for _, c := range colors {
		name := string(c)
		names = append(names, strings.ToUpper(name[:1])+name[1:])
	}
	label := widget.NewLabel(fmt.Sprintf("Remove %s labels from %d photos?", strings.Join(names, ", "), count))
	label.TextStyle = fyne.TextStyle{Bold: true}

	content := container.NewVBox(label)
	return container.New(&minWidthLayout{width: copyDialogWidth}, content)
}

type CopyAllDialog struct {
	dialog     *dialog.CustomDialog
	progress   *widget.ProgressBar
	destEntry  *DestinationEntry
	modeSelect *CopyModeSelect
	copyBtn    *widget.Button
	cancelBtn  *widget.Button
	onCopy     func()
	onCancel   func()
	closed     bool
	copying    bool
}

func NewCopyAllDialog(count int, destDir string, copyMode library.CopyMode, window fyne.Window, onCopy func(), onCancel func()) *CopyAllDialog {
	label := widget.NewLabel(fmt.Sprintf("Copy %d filtered photos", count))
	label.TextStyle = fyne.TextStyle{Bold: true}

	destEntry := NewDestinationEntry(destDir, window)

	modeSelect := NewCopyModeSelect(copyMode)

	progress := widget.NewProgressBar()
	progress.Hide()

	cad := &CopyAllDialog{
		progress:   progress,
		destEntry:  destEntry,
		modeSelect: modeSelect,
		onCopy:     onCopy,
		onCancel:   onCancel,
	}

	cad.copyBtn = widget.NewButton("Copy", func() {
		cad.copyBtn.Disable()
		cad.cancelBtn.SetText("Cancel")
		cad.progress.Show()
		if cad.onCopy != nil {
			cad.onCopy()
		}
	})
	cad.copyBtn.Importance = widget.HighImportance

	cad.cancelBtn = widget.NewButton("Close", func() {
		if cad.onCancel != nil {
			cad.onCancel()
		}
	})

	buttons := container.NewGridWithColumns(2, cad.cancelBtn, cad.copyBtn)

	content := container.NewVBox(label, destEntry.Container, modeSelect.radio, progress, buttons)
	wrapped := container.New(&minWidthLayout{width: copyDialogWidth}, content)

	cad.dialog = dialog.NewCustomWithoutButtons("Copy All", wrapped, window)
	cad.dialog.SetOnClosed(func() {
		if cad.closed {
			return
		}
		cad.closed = true
		if cad.onCancel != nil {
			cad.onCancel()
		}
	})

	return cad
}

func (d *CopyAllDialog) Show() {
	d.dialog.Show()
}

func (d *CopyAllDialog) Hide() {
	d.closed = true
	d.dialog.Hide()
}

func (d *CopyAllDialog) SetProgress(value float64) {
	d.progress.SetValue(value)
}

// CopyStarted freezes what the running copy already captured: destination and
// mode edits would silently be ignored, and a folder picker opened mid-copy
// would be torn down with the dialog when the goroutine closes it.
func (d *CopyAllDialog) CopyStarted() {
	d.copying = true
	d.destEntry.Disable()
	d.modeSelect.radio.Disable()
}

func (d *CopyAllDialog) Copying() bool {
	return d.copying
}

func (d *CopyAllDialog) Cancelling() {
	d.cancelBtn.SetText("Cancelling...")
	d.cancelBtn.Disable()
}

func (d *CopyAllDialog) DestDir() string {
	return d.destEntry.Text()
}

func (d *CopyAllDialog) CopyMode() library.CopyMode {
	return d.modeSelect.Mode
}
