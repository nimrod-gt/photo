package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

type ToolbarCallbacks struct {
	OnFavorite func()
	OnRed      func()
	OnGreen    func()
	OnBlue     func()
	OnDelete   func()
}

type Toolbar struct {
	container *fyne.Container
	callbacks ToolbarCallbacks
}

func NewToolbar(callbacks ToolbarCallbacks) *Toolbar {
	t := &Toolbar{callbacks: callbacks}
	t.build()
	return t
}

func (t *Toolbar) Container() *fyne.Container {
	return t.container
}

func (t *Toolbar) build() {
	favBtn := widget.NewButton("Favorite", func() {
		if t.callbacks.OnFavorite != nil {
			t.callbacks.OnFavorite()
		}
	})
	redBtn := widget.NewButton("Red", func() {
		if t.callbacks.OnRed != nil {
			t.callbacks.OnRed()
		}
	})
	greenBtn := widget.NewButton("Green", func() {
		if t.callbacks.OnGreen != nil {
			t.callbacks.OnGreen()
		}
	})
	blueBtn := widget.NewButton("Blue", func() {
		if t.callbacks.OnBlue != nil {
			t.callbacks.OnBlue()
		}
	})
	deleteBtn := widget.NewButton("Delete", func() {
		if t.callbacks.OnDelete != nil {
			t.callbacks.OnDelete()
		}
	})

	t.container = NewHBox(favBtn, redBtn, greenBtn, blueBtn, deleteBtn)
}
