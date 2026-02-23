package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type ContextMenuCallbacks struct {
	OnFavorite func()
	OnRed      func()
	OnGreen    func()
	OnBlue     func()
	OnDelete   func()
}

type ContextMenuItems struct {
	Menu     *fyne.Menu
	Favorite *fyne.MenuItem
	Red      *fyne.MenuItem
	Green    *fyne.MenuItem
	Blue     *fyne.MenuItem
}

func NewContextMenu(callbacks ContextMenuCallbacks) ContextMenuItems {
	favItem := fyne.NewMenuItemWithIcon("Favorite", iconHeart, func() {
		if callbacks.OnFavorite != nil {
			callbacks.OnFavorite()
		}
	})
	redItem := fyne.NewMenuItemWithIcon("Red", iconRedCircle, func() {
		if callbacks.OnRed != nil {
			callbacks.OnRed()
		}
	})
	greenItem := fyne.NewMenuItemWithIcon("Green", iconGreenCircle, func() {
		if callbacks.OnGreen != nil {
			callbacks.OnGreen()
		}
	})
	blueItem := fyne.NewMenuItemWithIcon("Blue", iconBlueCircle, func() {
		if callbacks.OnBlue != nil {
			callbacks.OnBlue()
		}
	})
	menu := fyne.NewMenu("",
		favItem,
		fyne.NewMenuItemSeparator(),
		redItem, greenItem, blueItem,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItemWithIcon("Delete", theme.DeleteIcon(), func() {
			if callbacks.OnDelete != nil {
				callbacks.OnDelete()
			}
		}),
	)
	return ContextMenuItems{
		Menu:     menu,
		Favorite: favItem,
		Red:      redItem,
		Green:    greenItem,
		Blue:     blueItem,
	}
}

func ShowContextMenu(menu *fyne.Menu, canvas fyne.Canvas, pos fyne.Position) {
	popup := widget.NewPopUpMenu(menu, canvas)
	popup.ShowAtPosition(pos)
}
