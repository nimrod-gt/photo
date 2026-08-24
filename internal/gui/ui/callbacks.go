package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

func call(fn func()) {
	if fn != nil {
		fn()
	}
}

// The callback is read once, when the button is built, so a widget whose
// callbacks arrive later has to wrap its own lookup instead of handing a field
// over here.
func iconButton(label string, icon fyne.Resource, fn func()) *widget.Button {
	return widget.NewButtonWithIcon(label, icon, func() {
		call(fn)
	})
}
