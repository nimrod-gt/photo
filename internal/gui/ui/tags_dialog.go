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

	"photo/internal/core/claudebin"
	"photo/internal/core/model"
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
	OnCancelRun    func()
	OnBackground   func()
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
	window          fyne.Window
	callbacks       TagsDialogCallbacks
	concept         *escapeEntry
	location        *escapeEntry
	split           model.Place
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
	cancelRunBtn    *widget.Button
	backgroundBtn   *widget.Button
	buttons         *fyne.Container
	generating      bool
	closed          bool
}

func NewTagsDialog(opts TagsDialogOptions, window fyne.Window, callbacks TagsDialogCallbacks) *TagsDialog {
	d := &TagsDialog{window: window, callbacks: callbacks, defaultDate: opts.Date}
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
		d.buttons,
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
		d.Generating()
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

	d.closeBtn = widget.NewButton("Close (ESC)", d.requestClose)

	d.cancelRunBtn = widget.NewButton("Cancel (N)", func() { call(d.callbacks.OnCancelRun) })
	d.cancelRunBtn.Importance = widget.DangerImportance
	d.backgroundBtn = widget.NewButton("Background (B)", func() { call(d.callbacks.OnBackground) })

	d.buttons = container.New(layout.NewGridLayout(1))
	d.setGenerating(false)
}

// A run offers no way out but its own two: closing the dialog would have to
// mean one of them anyway, and copying or saving tags that are still being
// generated has nothing to work with.
func (d *TagsDialog) buttonSet() []fyne.CanvasObject {
	if d.generating {
		return []fyne.CanvasObject{d.cancelRunBtn, d.backgroundBtn, d.generateBtn}
	}
	buttons := make([]fyne.CanvasObject, 0, 5)
	buttons = append(buttons, d.closeBtn, d.copyTitleBtn, d.copyKeywordsBtn)
	if d.saveJPEGBtn != nil {
		buttons = append(buttons, d.saveJPEGBtn)
	}
	return append(buttons, d.generateBtn)
}

func (d *TagsDialog) setGenerating(generating bool) {
	d.generating = generating
	buttons := d.buttonSet()
	d.buttons.Layout = layout.NewGridLayout(len(buttons))
	d.buttons.Objects = buttons
	d.buttons.Refresh()
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
	if location := d.Location(); len(location) != 0 {
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

func (d *TagsDialog) Location() string {
	return strings.TrimSpace(d.location.Text)
}

func (d *TagsDialog) Tags() model.Tags {
	return model.Tags{
		Title:    strings.TrimSpace(d.title.Text),
		Keywords: model.ParseKeywordLine(d.keywords.Text),
		Place:    d.place(),
	}
}

// The split into city, region and country is never shown, so a wrong one cannot
// be corrected by hand. It is kept only while the location it was made from is
// still the one in the entry: an edited location ships as free text alone rather
// than with a city that no longer belongs to it.
func (d *TagsDialog) place() model.Place {
	location := d.Location()
	if location != d.split.Location {
		return model.Place{Location: location}
	}
	return d.split
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
	if !d.resultUntouched() {
		return
	}
	// A file can carry a place and no tags at all - a Lightroom sidecar with a
	// location in it, or a photo whose location was typed and never generated
	// from. The location is the user's own field and is filled either way; the
	// result fields stay hidden while there is nothing to put in them.
	if existing.IsEmpty() {
		d.takePlace(existing.Place)
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
	d.takePlace(shown.Place)
	d.resultBox.Show()
	d.refreshStatus()
}

func (d *TagsDialog) takePlace(place model.Place) {
	d.split = place.Trimmed()
	// A location typed while the run was going is the newer one and stays; the
	// split then no longer matches it and place() drops it.
	if len(d.Location()) == 0 {
		d.location.SetText(d.split.Location)
	}
}

func (d *TagsDialog) Fail(err error) {
	d.finishRun()
	if errors.Is(err, claudebin.ErrNotFound) {
		d.pathRow.Show()
	}
	d.setStatus(err.Error())
}

// Generating is also how a dialog reopened over a run that is still going
// catches up with it, so the state it puts the dialog in belongs here rather
// than in the Generate button.
//
// The focus goes with it: Cancel and Background are reached by their letters as
// well, and a plain letter typed into an entry belongs to the entry, so the
// canvas only sees those keys while nothing is focused.
func (d *TagsDialog) Generating() {
	d.generateBtn.Disable()
	d.setStatus("Generating, this takes up to a minute...")
	d.progress.Show()
	d.progress.Start()
	d.setGenerating(true)
	d.window.Canvas().Unfocus()
}

func (d *TagsDialog) finishRun() {
	d.progress.Stop()
	d.progress.Hide()
	d.generateBtn.Enable()
	d.generateBtn.SetText("Regenerate")
	d.setGenerating(false)
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
	// While a run goes the status line is its own, and the tags read from the
	// file may land in the fields underneath it; the run rewrites the line when
	// it finishes either way.
	if d.generating {
		return
	}
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
