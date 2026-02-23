package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

type ContextMenuCallbacks struct {
	OnFavorite func()
	OnRed      func()
	OnGreen    func()
	OnBlue     func()
	OnDelete   func()
}

func NewContextMenu(callbacks ContextMenuCallbacks) (*fyne.Menu, *fyne.MenuItem) {
	favItem := fyne.NewMenuItem("Favorite", func() {
		if callbacks.OnFavorite != nil {
			callbacks.OnFavorite()
		}
	})
	menu := fyne.NewMenu("",
		favItem,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Red", func() {
			if callbacks.OnRed != nil {
				callbacks.OnRed()
			}
		}),
		fyne.NewMenuItem("Green", func() {
			if callbacks.OnGreen != nil {
				callbacks.OnGreen()
			}
		}),
		fyne.NewMenuItem("Blue", func() {
			if callbacks.OnBlue != nil {
				callbacks.OnBlue()
			}
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Delete", func() {
			if callbacks.OnDelete != nil {
				callbacks.OnDelete()
			}
		}),
	)
	return menu, favItem
}

func ShowContextMenu(menu *fyne.Menu, canvas fyne.Canvas, pos fyne.Position) {
	popup := widget.NewPopUpMenu(menu, canvas)
	popup.ShowAtPosition(pos)
}
