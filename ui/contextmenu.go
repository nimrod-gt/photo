package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type ContextMenuCallbacks struct {
	OnRed           func()
	OnGreen         func()
	OnBlue          func()
	OnDelete        func()
	OnCopyClipboard func()
}

type ContextMenuItems struct {
	Menu  *fyne.Menu
	Red   *fyne.MenuItem
	Green *fyne.MenuItem
	Blue  *fyne.MenuItem
}

func NewContextMenu(callbacks ContextMenuCallbacks) ContextMenuItems {
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
		redItem, greenItem, blueItem,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Copy to Clipboard", func() {
			if callbacks.OnCopyClipboard != nil {
				callbacks.OnCopyClipboard()
			}
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItemWithIcon("Delete", theme.DeleteIcon(), func() {
			if callbacks.OnDelete != nil {
				callbacks.OnDelete()
			}
		}),
	)
	return ContextMenuItems{
		Menu:  menu,
		Red:   redItem,
		Green: greenItem,
		Blue:  blueItem,
	}
}

func ShowContextMenu(menu *fyne.Menu, canvas fyne.Canvas, pos fyne.Position) {
	popup := widget.NewPopUpMenu(menu, canvas)
	popup.ShowAtPosition(pos)
}
