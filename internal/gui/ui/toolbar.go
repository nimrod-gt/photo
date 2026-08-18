package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"photo/internal/core/model"
)

type ActionPanelCallbacks struct {
	OnRed    func()
	OnGreen  func()
	OnBlue   func()
	OnDelete func()
}

type ActionPanel struct {
	container *fyne.Container
	callbacks ActionPanelCallbacks
	redBtn    *widget.Button
	greenBtn  *widget.Button
	blueBtn   *widget.Button
}

func NewActionPanel(callbacks ActionPanelCallbacks) *ActionPanel {
	p := &ActionPanel{callbacks: callbacks}
	p.build()
	return p
}

func (p *ActionPanel) Container() *fyne.Container {
	return p.container
}

func (p *ActionPanel) SetColorActive(label model.ColorLabel, active bool) {
	var btn *widget.Button
	switch label {
	case model.ColorRed:
		btn = p.redBtn
	case model.ColorGreen:
		btn = p.greenBtn
	case model.ColorBlue:
		btn = p.blueBtn
	default:
		return
	}
	if active {
		btn.Importance = widget.HighImportance
	} else {
		btn.Importance = widget.MediumImportance
	}
	btn.Refresh()
}

func (p *ActionPanel) build() {
	p.redBtn = widget.NewButtonWithIcon("Red", iconRedCircle, func() {
		if p.callbacks.OnRed != nil {
			p.callbacks.OnRed()
		}
	})
	p.greenBtn = widget.NewButtonWithIcon("Green", iconGreenCircle, func() {
		if p.callbacks.OnGreen != nil {
			p.callbacks.OnGreen()
		}
	})
	p.blueBtn = widget.NewButtonWithIcon("Blue", iconBlueCircle, func() {
		if p.callbacks.OnBlue != nil {
			p.callbacks.OnBlue()
		}
	})
	deleteBtn := widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {
		if p.callbacks.OnDelete != nil {
			p.callbacks.OnDelete()
		}
	})

	p.container = container.NewGridWithColumns(4, p.redBtn, p.greenBtn, p.blueBtn, deleteBtn)
}
