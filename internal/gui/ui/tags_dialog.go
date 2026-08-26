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
	"github.com/go-gl/glfw/v3.4/glfw"

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
	Keys       KeyMatcher
}

type TagsDialog struct {
	dialog          *dialog.CustomDialog
	window          fyne.Window
	callbacks       TagsDialogCallbacks
	keys            KeyMatcher
	concept         *dialogEntry
	location        *dialogEntry
	split           model.Place
	editorial       *dialogCheck
	date            *dialogDateEntry
	dateRow         *fyne.Container
	defaultDate     time.Time
	pathEntry       *dialogEntry
	pathRow         *fyne.Container
	progress        *widget.ProgressBarInfinite
	resultBox       *fyne.Container
	title           *dialogEntry
	keywords        *dialogEntry
	status          *widget.Label
	generateBtn     *dialogButton
	copyTitleBtn    *dialogButton
	copyKeywordsBtn *dialogButton
	saveJPEGBtn     *dialogButton
	closeBtn        *dialogButton
	cancelRunBtn    *dialogButton
	backgroundBtn   *dialogButton
	buttons         *fyne.Container
	generating      bool
	shown           bool
	closed          bool
}

func NewTagsDialog(opts TagsDialogOptions, window fyne.Window, callbacks TagsDialogCallbacks) *TagsDialog {
	d := &TagsDialog{window: window, callbacks: callbacks, keys: opts.Keys, defaultDate: opts.Date}
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
	d.concept = newDialogEntry(d.handleKey)
	d.concept.SetPlaceHolder("What the photo is about, optional")
	d.concept.OnSubmitted = d.submitted

	d.location = newDialogEntry(d.handleKey)
	d.location.SetPlaceHolder("City, country, optional")
	d.location.OnSubmitted = d.submitted

	d.date = newDialogDateEntry(d.handleKey)
	if !opts.Date.IsZero() {
		d.date.SetDate(&opts.Date)
	}
	d.dateRow = labeledRow("Date:", d.date)
	d.dateRow.Hide()

	d.editorial = newDialogCheck("Editorial", func(checked bool) {
		if checked {
			d.dateRow.Show()
			return
		}
		d.dateRow.Hide()
	}, d.handleCommandKey)

	d.pathEntry = newDialogEntry(d.handleKey)
	d.pathEntry.SetPlaceHolder("Path to the claude binary")
	d.pathEntry.SetText(opts.ClaudePath)
	d.pathEntry.OnSubmitted = d.submitted
	d.pathRow = labeledRow("claude:", d.pathEntry)
	d.pathRow.Hide()
}

func (d *TagsDialog) buildResult() {
	d.title = newDialogMultiLineEntry(titleRows, d.handleKey)
	d.title.SetPlaceHolder("Title")
	d.title.OnChanged = func(string) { d.refreshStatus() }

	d.keywords = newDialogMultiLineEntry(keywordRows, d.handleKey)
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
	d.generateBtn = newDialogButton("Generate", d.startGenerate, d.handleCommandKey)
	d.generateBtn.Importance = widget.HighImportance

	d.copyTitleBtn = newDialogButton("Copy title", func() { call(d.callbacks.OnCopyTitle) }, d.handleCommandKey)
	d.copyTitleBtn.Disable()

	d.copyKeywordsBtn = newDialogButton("Copy keywords", func() { call(d.callbacks.OnCopyKeywords) }, d.handleCommandKey)
	d.copyKeywordsBtn.Disable()

	if opts.IsJPEG {
		d.saveJPEGBtn = newDialogButton("Save JPEG", func() { call(d.callbacks.OnSaveJPEG) }, d.handleCommandKey)
		d.saveJPEGBtn.Disable()
	}

	d.closeBtn = newDialogButton("Close (ESC)", d.requestClose, d.handleCommandKey)

	d.cancelRunBtn = newDialogButton("Cancel (N)", func() { call(d.callbacks.OnCancelRun) }, d.handleCommandKey)
	d.cancelRunBtn.Importance = widget.DangerImportance
	d.backgroundBtn = newDialogButton("Background (B)", func() { call(d.callbacks.OnBackground) }, d.handleCommandKey)

	d.buttons = container.New(layout.NewGridLayout(1))
	d.setGenerating(false)
}

func (d *TagsDialog) submitted(string) {
	d.startGenerate()
}

// Enter starts a run from any of the single-line inputs, and the button leads
// here too, so a second run cannot be started over the one already going - the
// button is disabled then, and Enter would not know that on its own.
func (d *TagsDialog) startGenerate() {
	if d.generating || d.generateBtn.Disabled() {
		return
	}
	d.Generating()
	call(d.callbacks.OnGenerate)
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

// Every focusable widget of the dialog offers its keys here before handling
// them itself, so the keys the dialog owns work wherever the focus sits.
func (d *TagsDialog) handleKey(ev *fyne.KeyEvent) bool {
	if ev.Name == fyne.KeyEscape {
		d.requestEscape()
		return true
	}
	return false
}

// The two letters that command a run are only offered to the widgets that take
// no text: an entry is handed the rune of the key as well, so swallowing the
// key there would leave the letter behind in the field.
func (d *TagsDialog) handleCommandKey(ev *fyne.KeyEvent) bool {
	if d.handleKey(ev) {
		return true
	}
	if !d.generating {
		return false
	}
	switch {
	case d.keys.Matches(ev, glfw.KeyN, fyne.KeyN):
		call(d.callbacks.OnCancelRun)
	case d.keys.Matches(ev, glfw.KeyB, fyne.KeyB):
		call(d.callbacks.OnBackground)
	default:
		return false
	}
	return true
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
	d.shown = true
	d.focus(d.concept)
}

// The canvas turns down a widget that is not on screen and logs the refusal, so
// nothing is focused before the dialog is up.
func (d *TagsDialog) focus(obj fyne.Focusable) {
	if !d.shown {
		return
	}
	d.window.Canvas().Focus(obj)
}

func (d *TagsDialog) Hide() {
	d.closed = true
	d.shown = false
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
	d.focusAfterRun(d.title)
}

// A run lands up to a minute after it was started and the fields stay editable
// meanwhile, so the caret is only moved when it sits on one of the buttons the
// finished run takes out of the row and would otherwise leave it nowhere.
func (d *TagsDialog) focusAfterRun(next fyne.Focusable) {
	switch d.window.Canvas().Focused() {
	case nil, fyne.Focusable(d.cancelRunBtn), fyne.Focusable(d.backgroundBtn):
		d.focus(next)
	}
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
	next := fyne.Focusable(d.generateBtn)
	// The path is the one thing the user can do something about, so a run that
	// could not find the binary hands the keyboard straight to it.
	if errors.Is(err, claudebin.ErrNotFound) {
		d.pathRow.Show()
		next = d.pathEntry
	}
	d.setStatus(err.Error())
	d.focusAfterRun(next)
}

// Generating is also how a dialog reopened over a run that is still going
// catches up with it, so the state it puts the dialog in belongs here rather
// than in the Generate button.
//
// The focus goes to Cancel with it: it is the button a run is most likely to be
// interrupted by, and from there the letters of both buttons are read as the
// commands they are rather than as text.
func (d *TagsDialog) Generating() {
	d.generateBtn.Disable()
	d.setStatus("Generating, this takes up to a minute...")
	d.progress.Show()
	d.progress.Start()
	d.setGenerating(true)
	d.focus(d.cancelRunBtn)
}

// The button row is rebuilt here and the focus may be left on a button that is
// no longer in it, with nothing for the keyboard to walk from; both callers
// place it again through focusAfterRun once they have arranged what it can
// land on.
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
func setEnabled[T interface {
	comparable
	fyne.Disableable
}](button T, enabled bool) {
	var missing T
	switch {
	case button == missing:
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
