package ui

import (
	"image"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type gridItem struct {
	widget.BaseWidget
	thumb *canvas.Image
	name  *widget.Label
	root  fyne.CanvasObject
}

func newGridItem(tileWidth float32) *gridItem {
	thumb := canvas.NewImageFromImage(nil)
	thumb.SetMinSize(fyne.NewSize(tileWidth, tileWidth*gridThumbRatio))
	thumb.FillMode = canvas.ImageFillContain

	nameLabel := widget.NewLabel("placeholder")
	nameLabel.Truncation = fyne.TextTruncateEllipsis
	nameLabel.Alignment = fyne.TextAlignCenter

	item := &gridItem{
		thumb: thumb,
		name:  nameLabel,
		root:  container.NewBorder(nil, nameLabel, nil, nil, thumb),
	}
	item.ExtendBaseWidget(item)
	return item
}

func (gi *gridItem) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(gi.root)
}

func (gi *gridItem) update(name string, img image.Image) {
	updateThumbnail(gi.thumb, img)
	updateName(gi.name, name)
}
