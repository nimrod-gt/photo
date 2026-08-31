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

func callWith[T any](fn func(T), arg T) {
	if fn != nil {
		fn(arg)
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

func boldLabel(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.TextStyle = fyne.TextStyle{Bold: true}
	return label
}

// The overlay layouts size themselves to their container alone.
type zeroMinSize struct{}

func (zeroMinSize) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, 0)
}
