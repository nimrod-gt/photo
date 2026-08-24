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

func iconButton(label string, icon fyne.Resource, fn func()) *widget.Button {
	return widget.NewButtonWithIcon(label, icon, func() {
		call(fn)
	})
}
