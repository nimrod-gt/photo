package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type ActionPanelCallbacks struct {
	OnFavorite func()
	OnRed      func()
	OnGreen    func()
	OnBlue     func()
	OnDelete   func()
}

type ActionPanel struct {
	container *fyne.Container
	callbacks ActionPanelCallbacks
	favBtn    *widget.Button
}

func NewActionPanel(callbacks ActionPanelCallbacks) *ActionPanel {
	p := &ActionPanel{callbacks: callbacks}
	p.build()
	return p
}

func (p *ActionPanel) Container() *fyne.Container {
	return p.container
}

func (p *ActionPanel) SetFavoriteEnabled(enabled bool) {
	if enabled {
		p.favBtn.Enable()
	} else {
		p.favBtn.Disable()
	}
}

func (p *ActionPanel) SetFavoriteActive(active bool) {
	if active {
		p.favBtn.SetIcon(iconHeart)
		p.favBtn.Importance = widget.HighImportance
	} else {
		p.favBtn.SetIcon(iconHeartOutline)
		p.favBtn.Importance = widget.MediumImportance
	}
	p.favBtn.Refresh()
}

func (p *ActionPanel) build() {
	p.favBtn = widget.NewButtonWithIcon("Favorite", iconHeartOutline, func() {
		if p.callbacks.OnFavorite != nil {
			p.callbacks.OnFavorite()
		}
	})
	redBtn := widget.NewButtonWithIcon("Red", iconRedCircle, func() {
		if p.callbacks.OnRed != nil {
			p.callbacks.OnRed()
		}
	})
	greenBtn := widget.NewButtonWithIcon("Green", iconGreenCircle, func() {
		if p.callbacks.OnGreen != nil {
			p.callbacks.OnGreen()
		}
	})
	blueBtn := widget.NewButtonWithIcon("Blue", iconBlueCircle, func() {
		if p.callbacks.OnBlue != nil {
			p.callbacks.OnBlue()
		}
	})
	deleteBtn := widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {
		if p.callbacks.OnDelete != nil {
			p.callbacks.OnDelete()
		}
	})

	p.container = container.NewGridWithColumns(5, p.favBtn, redBtn, greenBtn, blueBtn, deleteBtn)
}
