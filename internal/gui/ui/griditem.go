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

// Smooth scaling costs a CatmullRom pass over the whole image plus a copy out
// of the NRGBA it produces, on the UI goroutine, every time a tile shows a
// different photo - which is what made the grid stutter while a row loaded.
// The images the grid caches are already about the size they are drawn at, so
// the GPU filters them well enough on its own. The browser rows keep the smooth
// default: their thumbnails are minified far more, and their list is nowhere
// near as hot.
func newGridItem(tileWidth float32) *gridItem {
	thumb := newThumbnailImage(tileWidth, tileWidth*gridThumbRatio)
	thumb.ScaleMode = canvas.ImageScaleFastest

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
