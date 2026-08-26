package ui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
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

	generateLabel   = "Generate (Shift+Enter)"
	regenerateLabel = "Regenerate (Shift+Enter)"
	backgroundLabel = "Background (Ctrl+Enter)"
)

type TagsDialogCallbacks struct {
	OnEscape       func()
	OnGenerate     func()
	OnStopRun      func()
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
	closeBtn        *unfocusableButton
	stopBtn         *unfocusableButton
	backgroundBtn   *dialogButton
	buttons         *fyne.Container
	generating      bool
	shiftDown       bool
	shown           bool
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
	d.concept = newDialogEntry(d)
	d.concept.SetPlaceHolder("What the photo is about, optional")

	d.location = newDialogEntry(d)
	d.location.SetPlaceHolder("City, country, optional")

	d.date = newDialogDateEntry(d)
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
	}, d)

	d.pathEntry = newDialogEntry(d)
	d.pathEntry.SetPlaceHolder("Path to the claude binary")
	d.pathEntry.SetText(opts.ClaudePath)
	d.pathRow = labeledRow("claude:", d.pathEntry)
	d.pathRow.Hide()
}

func (d *TagsDialog) buildResult() {
	d.title = newDialogMultiLineEntry(titleRows, d)
	d.title.SetPlaceHolder("Title")
	d.title.OnChanged = func(string) { d.refreshStatus() }

	d.keywords = newDialogMultiLineEntry(keywordRows, d)
	d.keywords.SetPlaceHolder("Keywords, comma separated")
	d.keywords.OnChanged = func(string) { d.refreshStatus() }

	d.copyTitleBtn = newCopyButton("Copy title", func() { call(d.callbacks.OnCopyTitle) }, d)
	d.copyKeywordsBtn = newCopyButton("Copy keywords", func() { call(d.callbacks.OnCopyKeywords) }, d)

	d.resultBox = container.NewVBox(
		copyableField(d.title, d.copyTitleBtn),
		copyableField(d.keywords, d.copyKeywordsBtn),
	)
	d.resultBox.Hide()

	d.status = widget.NewLabel("")
	d.status.Wrapping = fyne.TextWrapWord
	d.status.Hide()
}

func newCopyButton(label string, tapped func(), keys dialogKeys) *dialogButton {
	button := newDialogButton("", tapped, keys)
	button.label = label
	button.SetIcon(theme.ContentCopyIcon())
	button.Importance = widget.LowImportance
	button.Disable()
	return button
}

// The button keeps its own height beside a field several rows tall, so it sits
// in a box of its own at the top right of the field it copies.
func copyableField(field fyne.CanvasObject, copy fyne.CanvasObject) *fyne.Container {
	return container.NewBorder(nil, nil, nil, container.NewVBox(copy), field)
}

func labeledRow(text string, content fyne.CanvasObject) *fyne.Container {
	label := widget.NewLabel(text)
	sized := container.New(layout.NewGridWrapLayout(fyne.NewSize(tagsLabelWidth, label.MinSize().Height)), label)
	return container.NewBorder(nil, nil, sized, nil, content)
}

func (d *TagsDialog) buildButtons(opts TagsDialogOptions) {
	// The Generate button is the one the keyboard is most often left on, and a
	// high importance paints it in the primary colour - the very colour Fyne
	// blends in to show the focus, which would then be invisible on it.
	d.generateBtn = newDialogButton(generateLabel, d.startGenerate, d)

	if opts.IsJPEG {
		d.saveJPEGBtn = newDialogButton("Save JPEG", func() { call(d.callbacks.OnSaveJPEG) }, d)
		d.saveJPEGBtn.Disable()
	}

	d.closeBtn = newUnfocusableButton("Cancel (ESC)", d.requestClose)

	d.stopBtn = newUnfocusableButton("Stop (ESC)", func() { call(d.callbacks.OnStopRun) })
	d.stopBtn.Importance = widget.DangerImportance
	d.backgroundBtn = newDialogButton(backgroundLabel, d.startBackground, d)

	d.buttons = container.New(layout.NewGridLayout(1))
	d.setGenerating(false)
}

// Shift+Enter starts a run from wherever the focus sits, and the button leads
// here too, so a second run cannot be started over the one already going - the
// button is disabled then, and the chord would not know that on its own.
func (d *TagsDialog) startGenerate() {
	if d.generating || d.generateBtn.Disabled() {
		return
	}
	d.Generating()
	call(d.callbacks.OnGenerate)
}

// Leaving the dialog, generating and backgrounding are the same three places in
// the row whether a run is going or not, so a key learned in one state presses
// the button it is written on in the other. Writing into the JPEG is the one
// action a run has nothing to offer, since the tags it will bring are not there
// yet.
func (d *TagsDialog) buttonSet() []fyne.CanvasObject {
	if d.generating {
		return []fyne.CanvasObject{d.stopBtn, d.generateBtn, d.backgroundBtn}
	}
	buttons := make([]fyne.CanvasObject, 0, 4)
	buttons = append(buttons, d.closeBtn, d.generateBtn, d.backgroundBtn)
	if d.saveJPEGBtn != nil {
		buttons = append(buttons, d.saveJPEGBtn)
	}
	return buttons
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
	switch {
	case ev.Name == fyne.KeyEscape:
		d.requestEscape()
	case d.shiftDown && isReturn(ev.Name):
		d.startGenerate()
	default:
		return false
	}
	return true
}

// Ctrl+Enter is the one chord the driver reports as a shortcut. macOS names the
// Command key Super and leaves Ctrl as it is, so both count: the dialog asks
// for the place of the chord rather than for the key a platform calls its own.
func (d *TagsDialog) handleShortcut(shortcut fyne.Shortcut) bool {
	chord, ok := shortcut.(*desktop.CustomShortcut)
	if !ok || !isReturn(chord.KeyName) {
		return false
	}
	if chord.Modifier&(fyne.KeyModifierControl|fyne.KeyModifierSuper) == 0 {
		return false
	}
	d.startBackground()
	return true
}

// Shift is the only modifier the dialog has to remember: it never reaches a
// widget as part of the key it modifies.
func (d *TagsDialog) trackModifier(ev *fyne.KeyEvent, down bool) {
	if ev.Name == desktop.KeyShiftLeft || ev.Name == desktop.KeyShiftRight {
		d.shiftDown = down
	}
}

func isReturn(name fyne.KeyName) bool {
	return name == fyne.KeyReturn || name == fyne.KeyEnter
}

// Backgrounding means the same thing whether a run is going or not: the one
// that is going is let go of, and where there is none a run is started and let
// go of in the same breath.
func (d *TagsDialog) startBackground() {
	if !d.generating {
		d.startGenerate()
	}
	call(d.callbacks.OnBackground)
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
	d.shiftDown = false
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
	if concept := d.Concept(); len(concept) != 0 {
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

func (d *TagsDialog) Concept() string {
	return strings.TrimSpace(d.concept.Text)
}

func (d *TagsDialog) Tags() model.Tags {
	return model.Tags{
		Title:    strings.TrimSpace(d.title.Text),
		Keywords: model.ParseKeywordLine(d.keywords.Text),
		Place:    d.place(),
		Concept:  d.Concept(),
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
	// A file can carry a place or a concept and no tags at all - a Lightroom
	// sidecar with a location in it, or a photo whose note was typed and never
	// generated from. Those two are the user's own fields and are filled either
	// way; the result fields stay hidden while there is nothing to put in them.
	if existing.IsEmpty() {
		d.takePlace(existing.Place)
		d.takeConcept(existing.Concept)
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

// RestoreTags puts back what a dialog closing over a run handed to it, leaving
// the run itself alone: the fields are on screen and editable again while the
// generation that owns them still goes.
func (d *TagsDialog) RestoreTags(handed model.Tags) {
	d.showTags(handed)
}

// A run lands up to a minute after it was started and the fields stay editable
// meanwhile, so the caret is only moved when it sits on one of the buttons the
// finished run takes out of the row and would otherwise leave it nowhere.
func (d *TagsDialog) focusAfterRun(next fyne.Focusable) {
	switch d.window.Canvas().Focused() {
	case nil, fyne.Focusable(d.backgroundBtn):
		d.focus(next)
	}
}

func (d *TagsDialog) showTags(shown model.Tags) {
	d.title.SetText(shown.Title)
	d.keywords.SetText(shown.KeywordLine())
	d.takePlace(shown.Place)
	d.takeConcept(shown.Concept)
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

// Same rule as the location: a note typed while the run was going is the newer
// one and is never written over by what the run hands back.
func (d *TagsDialog) takeConcept(concept string) {
	if len(d.Concept()) == 0 {
		d.concept.SetText(strings.TrimSpace(concept))
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
func (d *TagsDialog) Generating() {
	d.generateBtn.Disable()
	d.setStatus("Generating, this takes up to a minute...")
	d.progress.Show()
	d.progress.Start()
	d.setGenerating(true)
	// The row is rebuilt around a run and the focus may be left where nothing
	// can be done with it: on Generate, which is disabled now and which the Tab
	// walk goes past, or on Save JPEG, which is out of the row while Space would
	// still press it. A mouse press on Generate leaves no focus at all, and the
	// dialog would then hear none of the chords its buttons are named after.
	switch d.window.Canvas().Focused() {
	case nil, fyne.Focusable(d.generateBtn), fyne.Focusable(d.saveJPEGBtn):
		d.focus(d.backgroundBtn)
	}
}

func (d *TagsDialog) IsGenerating() bool {
	return d.generating
}

// A stopped run brought no tags, so the Generate button keeps its own name and
// the line that announced the run gives way to what the fields say about
// themselves - which for tags that were there before the run is the count the
// status line held all along.
func (d *TagsDialog) StopGenerating() {
	d.endRun()
	d.refreshStatus()
	d.focusAfterRun(d.generateBtn)
}

// The button row is rebuilt here and the focus may be left on a button that is
// no longer in it, with nothing for the keyboard to walk from; both callers
// place it again through focusAfterRun once they have arranged what it can
// land on.
func (d *TagsDialog) finishRun() {
	d.endRun()
	d.generateBtn.SetText(regenerateLabel)
}

func (d *TagsDialog) endRun() {
	d.progress.Stop()
	d.progress.Hide()
	d.generateBtn.Enable()
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
