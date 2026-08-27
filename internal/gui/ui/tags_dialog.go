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
	OnCopyTags     func()
	OnPasteTags    func()
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
	dialog           *dialog.CustomDialog
	window           fyne.Window
	callbacks        TagsDialogCallbacks
	concept          *dialogEntry
	location         *dialogEntry
	split            model.Place
	editorial        *dialogCheck
	date             *dialogDateEntry
	dateRow          *fyne.Container
	defaultDate      time.Time
	pathEntry        *dialogEntry
	pathRow          *fyne.Container
	progress         *widget.ProgressBarInfinite
	resultBox        *fyne.Container
	title            *dialogEntry
	keywords         *dialogEntry
	status           *widget.Label
	generateBtn      *dialogButton
	copyTitleBtn     *dialogButton
	copyKeywordsBtn  *dialogButton
	saveJPEGBtn      *dialogButton
	closeBtn         *unfocusableButton
	stopBtn          *unfocusableButton
	backgroundBtn    *dialogButton
	buttons          *fyne.Container
	generating       bool
	editorialDecided bool
	dateDecided      bool
	lastScanCode     int
	shiftDown        bool
	shown            bool
	closed           bool
}

func NewTagsDialog(opts TagsDialogOptions, window fyne.Window, callbacks TagsDialogCallbacks) *TagsDialog {
	opts.Date = startOfDay(opts.Date)
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

	d.editorial = newDialogCheck("Editorial", d.editorialChanged, d)

	d.pathEntry = newDialogEntry(d)
	d.pathEntry.SetPlaceHolder("Path to the claude binary")
	d.pathEntry.SetText(opts.ClaudePath)
	d.pathRow = labeledRow("claude:", d.pathEntry)
	d.pathRow.Hide()
}

// A box that has been set once is answered for, whoever set it: the user, the
// mark the file carried, or the dialog that closed over a run. What lands after
// that is the older answer and leaves it alone.
func (d *TagsDialog) editorialChanged(checked bool) {
	if checked {
		d.dateRow.Show()
	} else {
		d.dateRow.Hide()
	}
	d.editorialDecided = true
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

// The chords the dialog answers wherever the focus sits. macOS names the
// Command key Super and leaves Ctrl as it is, so both count for the background
// chord: the dialog asks for the place of the chord rather than for the key a
// platform calls its own.
//
// Alt carries the two tag chords because Ctrl+C and Ctrl+V belong to the fields.
// Every widget of the dialog offers its shortcuts here before handling them
// itself, so taking those two would leave no way to copy a piece of a title.
func (d *TagsDialog) handleShortcut(shortcut fyne.Shortcut) bool {
	chord, ok := shortcut.(*desktop.CustomShortcut)
	if !ok {
		return false
	}
	switch {
	case isReturn(chord.KeyName) && chord.Modifier&(fyne.KeyModifierControl|fyne.KeyModifierSuper) != 0:
		d.startBackground()
	case chord.Modifier&fyne.KeyModifierAlt == 0:
		return false
	case d.chordOn(chord, fyne.KeyC, glfw.KeyC):
		call(d.callbacks.OnCopyTags)
	case d.chordOn(chord, fyne.KeyV, glfw.KeyV):
		call(d.callbacks.OnPasteTags)
	default:
		return false
	}
	return true
}

// Fyne names the key of a chord after the letter the layout prints on it, and
// only falls back to the ASCII key for the ctrl modifier, so an Alt chord typed
// in Cyrillic arrives with no name at all. The place of the key on the keyboard
// is what the rest of the app binds to, and the key going down brought it.
func (d *TagsDialog) chordOn(chord *desktop.CustomShortcut, name fyne.KeyName, place glfw.Key) bool {
	if chord.KeyName == name {
		return true
	}
	return chord.KeyName == fyne.KeyUnknown && d.lastScanCode != 0 && d.lastScanCode == keyScanCode(place)
}

// Shift is the only modifier the dialog has to remember: it never reaches a
// widget as part of the key it modifies. The scan code is remembered for the
// chords, which arrive without one.
func (d *TagsDialog) trackKey(ev *fyne.KeyEvent, down bool) {
	if ev.Name == desktop.KeyShiftLeft || ev.Name == desktop.KeyShiftRight {
		d.shiftDown = down
	}
	if down {
		d.lastScanCode = ev.Physical.ScanCode
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
	if editorial := d.Editorial(); editorial.Marked {
		lines = append(lines, strings.TrimSpace("Editorial: "+editorialDay(editorial.Date)))
	}
	return strings.Join(lines, "\n")
}

// The prompt is told the same value the file is given, rather than reading the
// widgets a second time: a day the two spell differently is a day the user sees
// in one place and finds in the other.
func editorialDay(date time.Time) string {
	if date.IsZero() {
		return ""
	}
	return date.Format(editorialDateLayout)
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
		Title:     strings.TrimSpace(d.title.Text),
		Keywords:  model.ParseKeywordLine(d.keywords.Text),
		Place:     d.place(),
		Concept:   d.Concept(),
		Editorial: d.Editorial(),
	}
}

// Editorial reports the mark as it stands on screen. An unticked box carries no
// day, however long one has been sitting in the entry behind it, and a ticked
// one with the entry cleared is a mark without a day rather than no mark at all.
func (d *TagsDialog) Editorial() model.Editorial {
	if !d.editorial.Checked {
		return model.Editorial{}
	}
	editorial := model.Editorial{Marked: true}
	if d.date.Date != nil {
		editorial.Date = *d.date.Date
	}
	return editorial.Normalized()
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
// them works the same either way, and the date it was marked for - or the day it
// was shot, when the file marks nothing - is only put in while the user has not
// picked a day of their own.
func (d *TagsDialog) SetPhotoInfo(existing model.Tags, taken time.Time) {
	d.seedDate(existing.Editorial, taken)
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
		d.takeEditorial(existing.Editorial)
		return
	}
	d.showTags(existing)
}

// The day the file was marked for is the one to show; the shooting date only
// stands in while the file names none. A file that marks the photo and names no
// day names none on purpose - the day was cleared in the dialog that wrote it -
// so the entry is emptied rather than filled with the hour the shutter went: a
// day nobody picked would ride out with the mark on the very next close.
func (d *TagsDialog) seedDate(editorial model.Editorial, taken time.Time) {
	if d.dateDecided || !d.dateUntouched() {
		return
	}
	editorial = editorial.Normalized()
	if editorial.Marked && editorial.Date.IsZero() {
		d.defaultDate = time.Time{}
		d.date.SetDate(nil)
		return
	}
	day := editorial.Date
	if day.IsZero() {
		day = startOfDay(taken)
	}
	if day.IsZero() {
		return
	}
	d.defaultDate = day
	d.date.SetDate(&day)
}

// The entry holds days, not moments: the shooting date carries the hour the
// shutter went, and a day seeded with 14:30 on it would read back as a day the
// user picked by hand and stop every later seed. The zone goes the same way and
// for the same reason - a shown entry parses its own text back, and text names
// no zone, so what comes back is UTC whatever went in - and the day is the day
// the moment names where it was written, not where it is read.
func startOfDay(moment time.Time) time.Time {
	if moment.IsZero() {
		return moment
	}
	year, month, day := moment.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
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

// The location and the concept are left exactly as they are: a run echoes back
// the free text it was asked with, so it has nothing to add to either, and
// filling them again would undo an edit made in the minute it took to answer -
// a note cleared while the run went would come straight back.
func (d *TagsDialog) SetTags(generated model.Tags) {
	d.finishRun()
	d.split = generated.Place.Trimmed()
	d.showResult(generated)
	d.focusAfterRun(d.title)
}

// RestoreTags puts back what a dialog closing over a run handed to it, leaving
// the run itself alone: the fields are on screen and editable again while the
// generation that owns them still goes.
//
// The dialog that closed handed over the whole of what it held, and held it
// later than the file did, so every field is put back over the seed this one
// read out of the cache a moment ago rather than only filling what that left
// empty.
func (d *TagsDialog) RestoreTags(handed model.Tags) {
	d.split = handed.Place.Trimmed()
	d.location.SetText(d.split.Location)
	d.concept.SetText(strings.TrimSpace(handed.Concept))
	d.restoreEditorial(handed.Editorial)
	d.showResult(handed)
}

// The handed mark stands whole, an unticked box and a cleared day included, and
// counts as the user's: a read landing after it is the older answer and must
// not tick a box the dialog that closed had cleared.
func (d *TagsDialog) restoreEditorial(editorial model.Editorial) {
	d.editorial.SetChecked(editorial.Marked)
	// An unmarked value put back over a box that was already clear changes
	// nothing and tells nobody, so the answer is recorded here rather than left
	// to the change handler.
	d.editorialDecided = true
	if !editorial.Marked {
		return
	}
	// The day is answered for by a value rather than by the entry standing on
	// one: a handed day the entry was already seeded with reads as untouched,
	// and the read landing behind it would put the day of the file in its place.
	d.dateDecided = true
	day := editorial.Date
	if day.IsZero() {
		d.date.SetDate(nil)
		return
	}
	d.date.SetDate(&day)
}

// PasteTags puts the tags copied out of another photo's dialog into the result
// fields and nothing else: the place, the note and the editorial mark belong to
// the photo in front of the user, not to the one the tags came from.
func (d *TagsDialog) PasteTags(pasted model.Tags) {
	d.showResult(pasted)
}

// HasTags says whether a paste would replace anything, which is what decides
// between pasting and asking first.
func (d *TagsDialog) HasTags() bool {
	return !d.resultUntouched()
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

// What the file already holds, which is the older of the two whenever the user
// has typed anything: it fills the free text fields only while they are empty.
func (d *TagsDialog) showTags(shown model.Tags) {
	d.takePlace(shown.Place)
	d.takeConcept(shown.Concept)
	d.takeEditorial(shown.Editorial)
	d.showResult(shown)
}

func (d *TagsDialog) showResult(shown model.Tags) {
	d.title.SetText(shown.Title)
	d.keywords.SetText(shown.KeywordLine())
	d.resultBox.Show()
	d.refreshStatus()
}

func (d *TagsDialog) takePlace(place model.Place) {
	d.split = place.Trimmed()
	// A location typed before the read landed is the newer one and stays; the
	// split then no longer matches it and place() drops it.
	if len(d.Location()) == 0 {
		d.location.SetText(d.split.Location)
	}
}

// Same rule as the location: a note typed before the read landed is the newer
// one and is never written over by what the file had.
func (d *TagsDialog) takeConcept(concept string) {
	if len(d.Concept()) == 0 {
		d.concept.SetText(strings.TrimSpace(concept))
	}
}

// Same rule again, with one half missing: no read ever unticks a box, because
// nothing in a file spells an unmarked photo - an absent mark is a file with
// nothing to say about one.
func (d *TagsDialog) takeEditorial(editorial model.Editorial) {
	if !editorial.Marked || d.editorialDecided {
		return
	}
	d.editorial.SetChecked(true)
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
