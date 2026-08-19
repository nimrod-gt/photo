package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// Fyne delivers key events to the focused widget and only falls back to the
// canvas handler when nothing holds focus, so a dialog full of inputs never
// sees Escape. These wrappers pass it back to the dialog.

type escapeEntry struct {
	widget.Entry
	onEscape func()
}

func newEscapeEntry(onEscape func()) *escapeEntry {
	e := &escapeEntry{onEscape: onEscape}
	e.ExtendBaseWidget(e)
	return e
}

func newEscapeMultiLineEntry(rows int, onEscape func()) *escapeEntry {
	e := &escapeEntry{onEscape: onEscape}
	e.MultiLine = true
	e.Wrapping = fyne.TextWrapWord
	e.ExtendBaseWidget(e)
	e.SetMinRowsVisible(rows)
	return e
}

func (e *escapeEntry) TypedKey(ev *fyne.KeyEvent) {
	if ev.Name == fyne.KeyEscape {
		e.onEscape()
		return
	}
	e.Entry.TypedKey(ev)
}

type escapeDateEntry struct {
	widget.DateEntry
	onEscape func()
}

func newEscapeDateEntry(onEscape func()) *escapeDateEntry {
	e := &escapeDateEntry{onEscape: onEscape}
	e.ExtendBaseWidget(e)
	e.Wrapping = fyne.TextWrap(fyne.TextTruncateClip)
	return e
}

func (e *escapeDateEntry) TypedKey(ev *fyne.KeyEvent) {
	if ev.Name == fyne.KeyEscape {
		e.onEscape()
		return
	}
	e.DateEntry.TypedKey(ev)
}

type escapeCheck struct {
	widget.Check
	onEscape func()
}

func newEscapeCheck(label string, changed func(bool), onEscape func()) *escapeCheck {
	c := &escapeCheck{onEscape: onEscape}
	c.Text = label
	c.OnChanged = changed
	c.ExtendBaseWidget(c)
	return c
}

func (c *escapeCheck) TypedKey(ev *fyne.KeyEvent) {
	if ev.Name == fyne.KeyEscape {
		c.onEscape()
		return
	}
	c.Check.TypedKey(ev)
}
