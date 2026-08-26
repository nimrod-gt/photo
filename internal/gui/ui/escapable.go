package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// Fyne delivers key events to the focused widget and only falls back to the
// canvas handler when nothing holds focus, so a dialog full of inputs never
// sees the keys it owns - Escape, and the chords that command a generation.
// These wrappers offer every key to the dialog first and forward the rest.
//
// A modifier arrives apart from the key it belongs to. A chord holding Ctrl
// comes as a shortcut, but Shift alone never does: the driver passes the bare
// key on and says nothing about the Shift, so the wrappers report the modifier
// keys going down and up as well. Every widget of the dialog reports them, so
// one held across a Shift+Tab - which the driver answers itself, without the
// widget hearing the Tab - is still known about where the focus lands.
type dialogKeys interface {
	handleKey(*fyne.KeyEvent) bool
	handleShortcut(fyne.Shortcut) bool
	trackModifier(ev *fyne.KeyEvent, down bool)
}

type dialogEntry struct {
	widget.Entry
	keys dialogKeys
	// The key that opened the dialog is still in flight while the dialog places
	// its first focus: the rune of that key is delivered afterwards and would
	// land in the field as text. Every real keystroke reaches the focused widget
	// as a key event before its rune does, so a rune with no key event in front
	// of it is the leftover one.
	strayRune bool
}

func newDialogEntry(keys dialogKeys) *dialogEntry {
	e := &dialogEntry{keys: keys}
	e.ExtendBaseWidget(e)
	return e
}

func newDialogMultiLineEntry(rows int, keys dialogKeys) *dialogEntry {
	e := &dialogEntry{keys: keys}
	e.MultiLine = true
	e.Wrapping = fyne.TextWrapWord
	e.ExtendBaseWidget(e)
	e.SetMinRowsVisible(rows)
	return e
}

// A multi-line Entry claims Tab for itself, and the driver asks this before it
// delivers the key, so the field would be a trap for the keyboard. Walking the
// dialog is worth more here than a literal tab inside a title or a keyword line.
func (e *dialogEntry) AcceptsTab() bool {
	return false
}

func (e *dialogEntry) TypedKey(ev *fyne.KeyEvent) {
	e.strayRune = false
	if e.keys.handleKey(ev) {
		return
	}
	e.Entry.TypedKey(ev)
}

func (e *dialogEntry) TypedShortcut(shortcut fyne.Shortcut) {
	if e.keys.handleShortcut(shortcut) {
		return
	}
	e.Entry.TypedShortcut(shortcut)
}

func (e *dialogEntry) KeyDown(ev *fyne.KeyEvent) {
	e.keys.trackModifier(ev, true)
	e.Entry.KeyDown(ev)
}

func (e *dialogEntry) KeyUp(ev *fyne.KeyEvent) {
	e.keys.trackModifier(ev, false)
	e.Entry.KeyUp(ev)
}

func (e *dialogEntry) FocusGained() {
	e.strayRune = true
	e.Entry.FocusGained()
}

func (e *dialogEntry) TypedRune(r rune) {
	if e.strayRune {
		e.strayRune = false
		return
	}
	e.Entry.TypedRune(r)
}

type dialogDateEntry struct {
	widget.DateEntry
	keys dialogKeys
}

func newDialogDateEntry(keys dialogKeys) *dialogDateEntry {
	e := &dialogDateEntry{keys: keys}
	e.ExtendBaseWidget(e)
	e.Wrapping = fyne.TextWrap(fyne.TextTruncateClip)
	return e
}

func (e *dialogDateEntry) TypedKey(ev *fyne.KeyEvent) {
	if e.keys.handleKey(ev) {
		return
	}
	e.DateEntry.TypedKey(ev)
}

func (e *dialogDateEntry) TypedShortcut(shortcut fyne.Shortcut) {
	if e.keys.handleShortcut(shortcut) {
		return
	}
	e.DateEntry.TypedShortcut(shortcut)
}

func (e *dialogDateEntry) KeyDown(ev *fyne.KeyEvent) {
	e.keys.trackModifier(ev, true)
	e.DateEntry.KeyDown(ev)
}

func (e *dialogDateEntry) KeyUp(ev *fyne.KeyEvent) {
	e.keys.trackModifier(ev, false)
	e.DateEntry.KeyUp(ev)
}

type dialogCheck struct {
	widget.Check
	keys dialogKeys
}

func newDialogCheck(label string, changed func(bool), keys dialogKeys) *dialogCheck {
	c := &dialogCheck{keys: keys}
	c.Text = label
	c.OnChanged = changed
	c.ExtendBaseWidget(c)
	return c
}

func (c *dialogCheck) TypedKey(ev *fyne.KeyEvent) {
	if c.keys.handleKey(ev) {
		return
	}
	c.Check.TypedKey(ev)
}

// A check takes no shortcut of its own, so what the dialog turns down is over.
func (c *dialogCheck) TypedShortcut(shortcut fyne.Shortcut) {
	c.keys.handleShortcut(shortcut)
}

func (c *dialogCheck) KeyDown(ev *fyne.KeyEvent) {
	c.keys.trackModifier(ev, true)
}

func (c *dialogCheck) KeyUp(ev *fyne.KeyEvent) {
	c.keys.trackModifier(ev, false)
}

type dialogButton struct {
	widget.Button
	keys dialogKeys
}

func newDialogButton(label string, tapped func(), keys dialogKeys) *dialogButton {
	b := &dialogButton{keys: keys}
	b.Text = label
	b.OnTapped = tapped
	b.ExtendBaseWidget(b)
	return b
}

// A Fyne button answers Space alone, and answers it through Tapped, which drops
// the canvas focus on its way past - the keyboard would lose the dialog on
// every press. Enter joins Space here and neither of them moves the focus.
func (b *dialogButton) TypedKey(ev *fyne.KeyEvent) {
	if b.keys.handleKey(ev) {
		return
	}
	switch ev.Name {
	case fyne.KeySpace, fyne.KeyReturn, fyne.KeyEnter:
		if !b.Disabled() {
			call(b.OnTapped)
		}
	}
}

func (b *dialogButton) TypedShortcut(shortcut fyne.Shortcut) {
	b.keys.handleShortcut(shortcut)
}

func (b *dialogButton) KeyDown(ev *fyne.KeyEvent) {
	b.keys.trackModifier(ev, true)
}

func (b *dialogButton) KeyUp(ev *fyne.KeyEvent) {
	b.keys.trackModifier(ev, false)
}

type unfocusableButton struct {
	widget.Button
}

func newUnfocusableButton(label string, tapped func()) *unfocusableButton {
	b := &unfocusableButton{}
	b.Text = label
	b.OnTapped = tapped
	b.ExtendBaseWidget(b)
	return b
}

// Escape presses this button, so the Tab walk has nothing to stop here for.
// Fyne has no way of being told that: the focus manager takes every visible
// widget implementing fyne.Focusable, so the interface is broken on purpose - a
// FocusGained of another shape hides the one the button would answer with.
func (b *unfocusableButton) FocusGained(bool) {}
