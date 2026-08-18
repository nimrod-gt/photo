package ui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"photo/internal/core/model"
	"photo/internal/core/tags"
)

const (
	tagsDialogWidth = float32(700)
	tagsLabelWidth  = float32(90)
	keywordRows     = 5
	// The prompt spells editorial dates out as "June 13, 2026", while the entry
	// itself shows and accepts the user's own locale format.
	editorialDateLayout = "January 2, 2006"
)

type TagsDialogCallbacks struct {
	OnGenerate func()
	OnCopy     func()
	OnClose    func()
}

type TagsDialogOptions struct {
	Filename   string
	ClaudePath string
	Date       time.Time
}

type TagsDialog struct {
	dialog      *dialog.CustomDialog
	callbacks   TagsDialogCallbacks
	concept     *widget.Entry
	location    *widget.Entry
	editorial   *widget.Check
	date        *widget.DateEntry
	dateRow     *fyne.Container
	defaultDate time.Time
	pathEntry   *widget.Entry
	pathRow     *fyne.Container
	existingBox *fyne.Container
	existing    *widget.Label
	progress    *widget.ProgressBarInfinite
	resultBox   *fyne.Container
	title       *widget.Entry
	keywords    *widget.Entry
	status      *widget.Label
	generateBtn *widget.Button
	copyBtn     *widget.Button
	closeBtn    *widget.Button
	closed      bool
}

func NewTagsDialog(opts TagsDialogOptions, window fyne.Window, callbacks TagsDialogCallbacks) *TagsDialog {
	d := &TagsDialog{callbacks: callbacks, defaultDate: opts.Date}
	d.build(opts, window)
	return d
}

func (d *TagsDialog) build(opts TagsDialogOptions, window fyne.Window) {
	nameLabel := widget.NewLabel(opts.Filename)
	nameLabel.TextStyle = fyne.TextStyle{Bold: true}

	d.buildInputs(opts)
	d.buildExisting()

	d.progress = widget.NewProgressBarInfinite()
	d.progress.Hide()

	d.buildResult()
	d.buildButtons()

	content := container.NewVBox(
		nameLabel,
		labeledRow("Concept:", d.concept),
		labeledRow("Location:", d.location),
		d.editorial,
		d.dateRow,
		d.pathRow,
		d.existingBox,
		d.progress,
		d.resultBox,
		d.status,
		container.NewGridWithColumns(3, d.closeBtn, d.copyBtn, d.generateBtn),
	)
	wrapped := container.New(&minWidthLayout{width: tagsDialogWidth}, content)

	d.dialog = dialog.NewCustomWithoutButtons("Stock Tags", wrapped, window)
	d.dialog.SetOnClosed(func() {
		if d.closed {
			return
		}
		d.closed = true
		if d.callbacks.OnClose != nil {
			d.callbacks.OnClose()
		}
	})
}

func (d *TagsDialog) buildInputs(opts TagsDialogOptions) {
	d.concept = widget.NewEntry()
	d.concept.SetPlaceHolder("What the photo is about, optional")

	d.location = widget.NewEntry()
	d.location.SetPlaceHolder("City, country, optional")

	d.date = widget.NewDateEntry()
	if !opts.Date.IsZero() {
		d.date.SetDate(&opts.Date)
	}
	d.dateRow = labeledRow("Date:", d.date)
	d.dateRow.Hide()

	d.editorial = widget.NewCheck("Editorial", func(checked bool) {
		if checked {
			d.dateRow.Show()
			return
		}
		d.dateRow.Hide()
	})

	d.pathEntry = widget.NewEntry()
	d.pathEntry.SetPlaceHolder("Path to the claude binary")
	d.pathEntry.SetText(opts.ClaudePath)
	d.pathRow = labeledRow("claude:", d.pathEntry)
	d.pathRow.Hide()
}

func (d *TagsDialog) buildResult() {
	d.title = widget.NewEntry()
	d.title.SetPlaceHolder("Title")
	d.title.OnChanged = func(string) { d.refreshStatus() }

	d.keywords = widget.NewMultiLineEntry()
	d.keywords.SetPlaceHolder("Keywords, comma separated")
	d.keywords.Wrapping = fyne.TextWrapWord
	d.keywords.SetMinRowsVisible(keywordRows)
	d.keywords.OnChanged = func(string) { d.refreshStatus() }

	d.resultBox = container.NewVBox(d.title, d.keywords)
	d.resultBox.Hide()

	d.status = widget.NewLabel("")
	d.status.Wrapping = fyne.TextWrapWord
	d.status.Hide()
}

func (d *TagsDialog) buildExisting() {
	header := widget.NewLabel("Existing tags")
	header.TextStyle = fyne.TextStyle{Bold: true}

	d.existing = widget.NewLabel("")
	d.existing.Wrapping = fyne.TextWrapWord

	d.existingBox = container.NewVBox(header, d.existing)
	d.existingBox.Hide()
}

func labeledRow(text string, content fyne.CanvasObject) *fyne.Container {
	label := widget.NewLabel(text)
	sized := container.New(layout.NewGridWrapLayout(fyne.NewSize(tagsLabelWidth, label.MinSize().Height)), label)
	return container.NewBorder(nil, nil, sized, nil, content)
}

func (d *TagsDialog) buildButtons() {
	d.generateBtn = widget.NewButton("Generate", func() {
		d.generateBtn.Disable()
		d.setStatus("Generating, this takes up to a minute...")
		d.progress.Show()
		d.progress.Start()
		if d.callbacks.OnGenerate != nil {
			d.callbacks.OnGenerate()
		}
	})
	d.generateBtn.Importance = widget.HighImportance

	d.copyBtn = widget.NewButton("Copy", func() {
		if d.callbacks.OnCopy != nil {
			d.callbacks.OnCopy()
		}
	})
	d.copyBtn.Disable()

	d.closeBtn = widget.NewButton("Close", func() {
		if d.callbacks.OnClose != nil {
			d.callbacks.OnClose()
		}
	})
}

func (d *TagsDialog) Show() {
	d.dialog.Show()
}

func (d *TagsDialog) Hide() {
	d.closed = true
	d.progress.Stop()
	d.dialog.Hide()
}

// Notes renders the optional inputs in the input format the prompt expects.
func (d *TagsDialog) Notes() string {
	var lines []string
	if concept := strings.TrimSpace(d.concept.Text); len(concept) != 0 {
		lines = append(lines, "Concept: "+concept)
	}
	if location := strings.TrimSpace(d.location.Text); len(location) != 0 {
		lines = append(lines, "Location: "+location)
	}
	if d.editorial.Checked {
		lines = append(lines, strings.TrimSpace("Editorial: "+d.editorialDate()))
	}
	return strings.Join(lines, "\n")
}

func (d *TagsDialog) editorialDate() string {
	if d.date.Date == nil {
		return ""
	}
	return d.date.Date.Format(editorialDateLayout)
}

func (d *TagsDialog) ClaudePath() string {
	return strings.TrimSpace(d.pathEntry.Text)
}

func (d *TagsDialog) Tags() model.Tags {
	return model.Tags{
		Title:    strings.TrimSpace(d.title.Text),
		Keywords: model.ParseKeywordLine(d.keywords.Text),
	}
}

// SetPhotoInfo fills in what the file itself already knows: the tags written to
// it earlier and the shooting date, which is only kept while the user has not
// typed a date of their own.
func (d *TagsDialog) SetPhotoInfo(existing model.Tags, taken time.Time) {
	if !taken.IsZero() && d.dateUntouched() {
		d.defaultDate = taken
		d.date.SetDate(&taken)
	}
	lines := make([]string, 0, 2)
	if title := strings.TrimSpace(existing.Title); len(title) != 0 {
		lines = append(lines, title)
	}
	if len(existing.Keywords) != 0 {
		lines = append(lines, existing.KeywordLine())
	}
	if len(lines) == 0 {
		return
	}
	d.existing.SetText(strings.Join(lines, "\n"))
	d.existingBox.Show()
}

func (d *TagsDialog) dateUntouched() bool {
	if d.date.Date == nil {
		return d.defaultDate.IsZero()
	}
	return d.date.Date.Equal(d.defaultDate)
}

func (d *TagsDialog) SetTags(tags model.Tags) {
	d.finishRun()
	d.title.SetText(tags.Title)
	d.keywords.SetText(tags.KeywordLine())
	d.resultBox.Show()
	d.refreshStatus()
}

func (d *TagsDialog) Fail(err error) {
	d.finishRun()
	if errors.Is(err, tags.ErrClaudeNotFound) {
		d.pathRow.Show()
	}
	d.setStatus(err.Error())
}

func (d *TagsDialog) finishRun() {
	d.progress.Stop()
	d.progress.Hide()
	d.generateBtn.Enable()
	d.generateBtn.SetText("Regenerate")
}

func (d *TagsDialog) refreshStatus() {
	tags := d.Tags()
	if len(tags.Title) == 0 && len(tags.Keywords) == 0 {
		d.copyBtn.Disable()
		d.setStatus("")
		return
	}
	d.copyBtn.Enable()
	if problems := tags.Problems(); len(problems) > 0 {
		d.setStatus(strings.Join(problems, "; "))
		return
	}
	d.setStatus(fmt.Sprintf("%d keywords, ready to upload", len(tags.Keywords)))
}

func (d *TagsDialog) setStatus(text string) {
	d.status.SetText(text)
	if len(text) == 0 {
		d.status.Hide()
		return
	}
	d.status.Show()
}
