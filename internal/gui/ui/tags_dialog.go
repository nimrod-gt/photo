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
	tagsDialogWidth = float32(820)
	tagsLabelWidth  = float32(90)
	keywordRows     = 5
	titleRows       = 2
	// The prompt spells editorial dates out as "June 13, 2026", while the entry
	// itself shows and accepts the user's own locale format.
	editorialDateLayout = "January 2, 2006"
)

type TagsDialogCallbacks struct {
	OnEscape       func()
	OnGenerate     func()
	OnCopyTitle    func()
	OnCopyKeywords func()
	OnSaveJPEG     func()
	OnClose        func()
}

type TagsDialogOptions struct {
	Filename   string
	ClaudePath string
	Date       time.Time
	IsJPEG     bool
}

type TagsDialog struct {
	dialog          *dialog.CustomDialog
	callbacks       TagsDialogCallbacks
	concept         *escapeEntry
	location        *escapeEntry
	editorial       *escapeCheck
	date            *escapeDateEntry
	dateRow         *fyne.Container
	defaultDate     time.Time
	pathEntry       *escapeEntry
	pathRow         *fyne.Container
	progress        *widget.ProgressBarInfinite
	resultBox       *fyne.Container
	title           *escapeEntry
	keywords        *escapeEntry
	status          *widget.Label
	generateBtn     *widget.Button
	copyTitleBtn    *widget.Button
	copyKeywordsBtn *widget.Button
	saveJPEGBtn     *widget.Button
	closeBtn        *widget.Button
	closed          bool
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

	d.progress = widget.NewProgressBarInfinite()
	d.progress.Hide()

	d.buildResult()
	d.buildButtons(opts)

	content := container.NewVBox(
		nameLabel,
		labeledRow("Concept:", d.concept),
		labeledRow("Location:", d.location),
		d.editorial,
		d.dateRow,
		d.pathRow,
		d.progress,
		d.resultBox,
		d.status,
		d.buttonRow(),
	)
	wrapped := container.New(&minWidthLayout{width: tagsDialogWidth}, content)

	d.dialog = dialog.NewCustomWithoutButtons("Stock Tags", wrapped, window)
	d.dialog.SetOnClosed(func() {
		if d.closed {
			return
		}
		d.closed = true
		call(d.callbacks.OnClose)
	})
}

func (d *TagsDialog) buildInputs(opts TagsDialogOptions) {
	d.concept = newEscapeEntry(d.requestEscape)
	d.concept.SetPlaceHolder("What the photo is about, optional")

	d.location = newEscapeEntry(d.requestEscape)
	d.location.SetPlaceHolder("City, country, optional")

	d.date = newEscapeDateEntry(d.requestEscape)
	if !opts.Date.IsZero() {
		d.date.SetDate(&opts.Date)
	}
	d.dateRow = labeledRow("Date:", d.date)
	d.dateRow.Hide()

	d.editorial = newEscapeCheck("Editorial", func(checked bool) {
		if checked {
			d.dateRow.Show()
			return
		}
		d.dateRow.Hide()
	}, d.requestEscape)

	d.pathEntry = newEscapeEntry(d.requestEscape)
	d.pathEntry.SetPlaceHolder("Path to the claude binary")
	d.pathEntry.SetText(opts.ClaudePath)
	d.pathRow = labeledRow("claude:", d.pathEntry)
	d.pathRow.Hide()
}

func (d *TagsDialog) buildResult() {
	d.title = newEscapeMultiLineEntry(titleRows, d.requestEscape)
	d.title.SetPlaceHolder("Title")
	d.title.OnChanged = func(string) { d.refreshStatus() }

	d.keywords = newEscapeMultiLineEntry(keywordRows, d.requestEscape)
	d.keywords.SetPlaceHolder("Keywords, comma separated")
	d.keywords.OnChanged = func(string) { d.refreshStatus() }

	d.resultBox = container.NewVBox(d.title, d.keywords)
	d.resultBox.Hide()

	d.status = widget.NewLabel("")
	d.status.Wrapping = fyne.TextWrapWord
	d.status.Hide()
}

func labeledRow(text string, content fyne.CanvasObject) *fyne.Container {
	label := widget.NewLabel(text)
	sized := container.New(layout.NewGridWrapLayout(fyne.NewSize(tagsLabelWidth, label.MinSize().Height)), label)
	return container.NewBorder(nil, nil, sized, nil, content)
}

func (d *TagsDialog) buildButtons(opts TagsDialogOptions) {
	d.generateBtn = widget.NewButton("Generate", func() {
		d.generateBtn.Disable()
		d.setStatus("Generating, this takes up to a minute...")
		d.progress.Show()
		d.progress.Start()
		call(d.callbacks.OnGenerate)
	})
	d.generateBtn.Importance = widget.HighImportance

	d.copyTitleBtn = widget.NewButton("Copy title", func() { call(d.callbacks.OnCopyTitle) })
	d.copyTitleBtn.Disable()

	d.copyKeywordsBtn = widget.NewButton("Copy keywords", func() { call(d.callbacks.OnCopyKeywords) })
	d.copyKeywordsBtn.Disable()

	if opts.IsJPEG {
		d.saveJPEGBtn = widget.NewButton("Save JPEG", func() { call(d.callbacks.OnSaveJPEG) })
		d.saveJPEGBtn.Disable()
	}

	d.closeBtn = widget.NewButton("Close", d.requestClose)
}

func (d *TagsDialog) buttonRow() *fyne.Container {
	buttons := make([]fyne.CanvasObject, 0, 5)
	buttons = append(buttons, d.closeBtn, d.copyTitleBtn, d.copyKeywordsBtn)
	if d.saveJPEGBtn != nil {
		buttons = append(buttons, d.saveJPEGBtn)
	}
	buttons = append(buttons, d.generateBtn)
	return container.NewGridWithColumns(len(buttons), buttons...)
}

func (d *TagsDialog) requestClose() {
	call(d.callbacks.OnClose)
}

// Escape is answered by the app rather than here, because a Fyne popup - the
// calendar of the date entry - stacks its own overlay on top while the entry
// below keeps the focus, and only the app can tell that it is there. The Close
// button closes regardless.
func (d *TagsDialog) requestEscape() {
	if d.callbacks.OnEscape != nil {
		d.callbacks.OnEscape()
		return
	}
	d.requestClose()
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
// it earlier go into the very fields a run would fill, so editing and saving
// them works the same either way, and the shooting date is only kept while the
// user has not typed a date of their own.
func (d *TagsDialog) SetPhotoInfo(existing model.Tags, taken time.Time) {
	if !taken.IsZero() && d.dateUntouched() {
		d.defaultDate = taken
		d.date.SetDate(&taken)
	}
	if existing.IsEmpty() || !d.resultUntouched() {
		return
	}
	d.showTags(existing)
}

func (d *TagsDialog) resultUntouched() bool {
	return len(d.title.Text) == 0 && len(d.keywords.Text) == 0
}

func (d *TagsDialog) dateUntouched() bool {
	if d.date.Date == nil {
		return d.defaultDate.IsZero()
	}
	return d.date.Date.Equal(d.defaultDate)
}

func (d *TagsDialog) SetTags(generated model.Tags) {
	d.finishRun()
	d.showTags(generated)
}

func (d *TagsDialog) showTags(shown model.Tags) {
	d.title.SetText(shown.Title)
	d.keywords.SetText(shown.KeywordLine())
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
	current := d.Tags()
	problems := current.Problems()
	setEnabled(d.copyTitleBtn, len(current.Title) != 0)
	setEnabled(d.copyKeywordsBtn, len(current.Keywords) != 0)
	// Writing into the JPEG is the one action here that changes the photo itself,
	// so it stays shut until the tags meet every stock requirement - the status
	// line below already spells out what is missing. Nothing is lost meanwhile:
	// the sidecar is written whatever state the fields are in.
	setEnabled(d.saveJPEGBtn, len(problems) == 0)
	if len(current.Title) == 0 && len(current.Keywords) == 0 {
		d.setStatus("")
		return
	}
	if len(problems) != 0 {
		d.setStatus(strings.Join(problems, "; "))
		return
	}
	d.setStatus(fmt.Sprintf("%d keywords, ready to upload", len(current.Keywords)))
}

// A photo with no JPEG to write into has no button to enable.
func setEnabled(button *widget.Button, enabled bool) {
	switch {
	case button == nil:
	case enabled:
		button.Enable()
	default:
		button.Disable()
	}
}

func (d *TagsDialog) setStatus(text string) {
	d.status.SetText(text)
	if len(text) == 0 {
		d.status.Hide()
		return
	}
	d.status.Show()
}
