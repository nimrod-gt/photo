package ui

import (
	"fmt"

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

type CopyAllDialog struct {
	dialog    *dialog.CustomDialog
	progress  *widget.ProgressBar
	destEntry *DestinationEntry
	rawCheck  *RawCheck
	copyBtn   *widget.Button
	cancelBtn *widget.Button
	onCopy    func()
	onCancel  func()
	closed    bool
}

func NewCopyAllDialog(count int, destDir string, includeRAW bool, window fyne.Window, onCopy func(), onCancel func()) *CopyAllDialog {
	label := widget.NewLabel(fmt.Sprintf("Copy %d filtered photos", count))
	label.TextStyle = fyne.TextStyle{Bold: true}

	destEntry := NewDestinationEntry(destDir, window)

	rawCheck := NewRawCheck(includeRAW)

	progress := widget.NewProgressBar()
	progress.Hide()

	cad := &CopyAllDialog{
		progress:  progress,
		destEntry: destEntry,
		rawCheck:  rawCheck,
		onCopy:    onCopy,
		onCancel:  onCancel,
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

	content := container.NewVBox(label, destEntry.Container, rawCheck.check, progress, buttons)
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

func (d *CopyAllDialog) Finish() {
	d.Hide()
}

func (d *CopyAllDialog) DestDir() string {
	return d.destEntry.Text()
}

func (d *CopyAllDialog) IncludeRAW() bool {
	return d.rawCheck.Checked
}
