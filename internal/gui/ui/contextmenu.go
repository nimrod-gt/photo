package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type ContextMenuCallbacks struct {
	OnFavorite      func()
	OnRed           func()
	OnGreen         func()
	OnBlue          func()
	OnDelete        func()
	OnCopyClipboard func()
	OnTags          func()
}

type ContextMenuItems struct {
	Menu     *fyne.Menu
	Favorite *fyne.MenuItem
	Red      *fyne.MenuItem
	Green    *fyne.MenuItem
	Blue     *fyne.MenuItem
}

func NewContextMenu(callbacks ContextMenuCallbacks) ContextMenuItems {
	favoriteItem := fyne.NewMenuItemWithIcon("Favorite", iconHeart, func() { call(callbacks.OnFavorite) })
	redItem := fyne.NewMenuItemWithIcon("Red", iconRedCircle, func() { call(callbacks.OnRed) })
	greenItem := fyne.NewMenuItemWithIcon("Green", iconGreenCircle, func() { call(callbacks.OnGreen) })
	blueItem := fyne.NewMenuItemWithIcon("Blue", iconBlueCircle, func() { call(callbacks.OnBlue) })
	menu := fyne.NewMenu("",
		favoriteItem,
		fyne.NewMenuItemSeparator(),
		redItem, greenItem, blueItem,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Copy to Clipboard", func() { call(callbacks.OnCopyClipboard) }),
		fyne.NewMenuItem("Generate Tags", func() { call(callbacks.OnTags) }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItemWithIcon("Delete", theme.DeleteIcon(), func() { call(callbacks.OnDelete) }),
	)
	return ContextMenuItems{
		Menu:     menu,
		Favorite: favoriteItem,
		Red:      redItem,
		Green:    greenItem,
		Blue:     blueItem,
	}
}

func ShowContextMenu(menu *fyne.Menu, canvas fyne.Canvas, pos fyne.Position) {
	popup := widget.NewPopUpMenu(menu, canvas)
	popup.ShowAtPosition(pos)
}
