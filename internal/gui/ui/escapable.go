package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// Fyne delivers key events to the focused widget and only falls back to the
// canvas handler when nothing holds focus, so a dialog full of inputs never
// sees the keys it owns - Escape, and the letters that command a running
// generation. These wrappers offer every key to the dialog first and forward
// the rest.

type dialogEntry struct {
	widget.Entry
	onKey func(*fyne.KeyEvent) bool
	// The key that opened the dialog is still in flight while the dialog places
	// its first focus: the rune of that key is delivered afterwards and would
	// land in the field as text. Every real keystroke reaches the focused widget
	// as a key event before its rune does, so a rune with no key event in front
	// of it is the leftover one.
	strayRune bool
}

func newDialogEntry(onKey func(*fyne.KeyEvent) bool) *dialogEntry {
	e := &dialogEntry{onKey: onKey}
	e.ExtendBaseWidget(e)
	return e
}

func newDialogMultiLineEntry(rows int, onKey func(*fyne.KeyEvent) bool) *dialogEntry {
	e := &dialogEntry{onKey: onKey}
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
	if e.onKey(ev) {
		return
	}
	e.Entry.TypedKey(ev)
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
	onKey func(*fyne.KeyEvent) bool
}

func newDialogDateEntry(onKey func(*fyne.KeyEvent) bool) *dialogDateEntry {
	e := &dialogDateEntry{onKey: onKey}
	e.ExtendBaseWidget(e)
	e.Wrapping = fyne.TextWrap(fyne.TextTruncateClip)
	return e
}

func (e *dialogDateEntry) TypedKey(ev *fyne.KeyEvent) {
	if e.onKey(ev) {
		return
	}
	e.DateEntry.TypedKey(ev)
}

type dialogCheck struct {
	widget.Check
	onKey func(*fyne.KeyEvent) bool
}

func newDialogCheck(label string, changed func(bool), onKey func(*fyne.KeyEvent) bool) *dialogCheck {
	c := &dialogCheck{onKey: onKey}
	c.Text = label
	c.OnChanged = changed
	c.ExtendBaseWidget(c)
	return c
}

func (c *dialogCheck) TypedKey(ev *fyne.KeyEvent) {
	if c.onKey(ev) {
		return
	}
	c.Check.TypedKey(ev)
}

type dialogButton struct {
	widget.Button
	onKey func(*fyne.KeyEvent) bool
}

func newDialogButton(label string, tapped func(), onKey func(*fyne.KeyEvent) bool) *dialogButton {
	b := &dialogButton{onKey: onKey}
	b.Text = label
	b.OnTapped = tapped
	b.ExtendBaseWidget(b)
	return b
}

// A Fyne button answers Space alone, and answers it through Tapped, which drops
// the canvas focus on its way past - the keyboard would lose the dialog on
// every press. Enter joins Space here and neither of them moves the focus.
func (b *dialogButton) TypedKey(ev *fyne.KeyEvent) {
	if b.onKey(ev) {
		return
	}
	switch ev.Name {
	case fyne.KeySpace, fyne.KeyReturn, fyne.KeyEnter:
		if !b.Disabled() {
			call(b.OnTapped)
		}
	}
}
