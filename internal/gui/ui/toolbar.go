package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"photo/internal/core/model"
)

// The six actions of a photo, shared by every place that offers them - the
// toolbar, the context menu and the shortcuts - so a new action is wired once.
type PhotoActions struct {
	OnFavorite func()
	OnRed      func()
	OnGreen    func()
	OnBlue     func()
	OnTags     func()
	OnDelete   func()
}

type ActionPanel struct {
	container *fyne.Container
	callbacks PhotoActions
	favBtn    *widget.Button
	colorBtns map[model.ColorLabel]*widget.Button
}

func NewActionPanel(callbacks PhotoActions) *ActionPanel {
	p := &ActionPanel{callbacks: callbacks}
	p.build()
	return p
}

func (p *ActionPanel) Container() *fyne.Container {
	return p.container
}

func (p *ActionPanel) SetFavoriteEnabled(enabled bool) {
	setEnabled(p.favBtn, enabled)
}

func (p *ActionPanel) SetFavoriteActive(active bool) {
	if active {
		p.favBtn.Importance = widget.HighImportance
		p.favBtn.SetIcon(iconHeart)
	} else {
		p.favBtn.Importance = widget.MediumImportance
		p.favBtn.SetIcon(iconHeartOutline)
	}
}

func (p *ActionPanel) SetColorActive(label model.ColorLabel, active bool) {
	btn, ok := p.colorBtns[label]
	if !ok {
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
	p.favBtn = iconButton("Favorite", iconHeartOutline, p.callbacks.OnFavorite)
	p.colorBtns = map[model.ColorLabel]*widget.Button{
		model.ColorRed:   iconButton("Red", iconRedCircle, p.callbacks.OnRed),
		model.ColorGreen: iconButton("Green", iconGreenCircle, p.callbacks.OnGreen),
		model.ColorBlue:  iconButton("Blue", iconBlueCircle, p.callbacks.OnBlue),
	}
	tagsBtn := iconButton("Tags", theme.DocumentCreateIcon(), p.callbacks.OnTags)
	deleteBtn := iconButton("Delete", theme.DeleteIcon(), p.callbacks.OnDelete)

	p.container = container.NewGridWithColumns(6, p.favBtn,
		p.colorBtns[model.ColorRed], p.colorBtns[model.ColorGreen], p.colorBtns[model.ColorBlue],
		tagsBtn, deleteBtn)
}
