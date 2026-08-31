package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"photo/internal/core/model"
)

type ContextMenuCallbacks struct {
	PhotoActions
	OnCopyClipboard func()
}

type ContextMenuItems struct {
	Menu     *fyne.Menu
	Favorite *fyne.MenuItem
	Colors   map[model.ColorLabel]*fyne.MenuItem
}

func NewContextMenu(callbacks ContextMenuCallbacks) ContextMenuItems {
	favoriteItem := fyne.NewMenuItemWithIcon("Favorite", iconHeart, func() { call(callbacks.OnFavorite) })
	colors := map[model.ColorLabel]*fyne.MenuItem{
		model.ColorRed:   fyne.NewMenuItemWithIcon("Red", iconRedCircle, func() { call(callbacks.OnRed) }),
		model.ColorGreen: fyne.NewMenuItemWithIcon("Green", iconGreenCircle, func() { call(callbacks.OnGreen) }),
		model.ColorBlue:  fyne.NewMenuItemWithIcon("Blue", iconBlueCircle, func() { call(callbacks.OnBlue) }),
	}
	menu := fyne.NewMenu("",
		favoriteItem,
		fyne.NewMenuItemSeparator(),
		colors[model.ColorRed], colors[model.ColorGreen], colors[model.ColorBlue],
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Copy to Clipboard", func() { call(callbacks.OnCopyClipboard) }),
		fyne.NewMenuItem("Generate Tags", func() { call(callbacks.OnTags) }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItemWithIcon("Delete", theme.DeleteIcon(), func() { call(callbacks.OnDelete) }),
	)
	return ContextMenuItems{
		Menu:     menu,
		Favorite: favoriteItem,
		Colors:   colors,
	}
}

func ShowContextMenu(menu *fyne.Menu, canvas fyne.Canvas, pos fyne.Position) {
	popup := widget.NewPopUpMenu(menu, canvas)
	popup.ShowAtPosition(pos)
}
