package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/storage"

	"photo/model"
)

const colorIndicatorSize = 12

type ViewerCallbacks struct {
	OnTapped          func()
	OnSecondaryTapped func(pos fyne.Position)
}

type Viewer struct {
	container  *fyne.Container
	image      *canvas.Image
	indicators *fyne.Container
	callbacks  ViewerCallbacks
}

func NewViewer(callbacks ViewerCallbacks) *Viewer {
	v := &Viewer{callbacks: callbacks}
	v.build()
	return v
}

func (v *Viewer) Container() *fyne.Container {
	return v.container
}

func (v *Viewer) ShowPhoto(photo model.Photo) {
	uri := storage.NewFileURI(photo.JPEGPath)
	v.image.Resource = nil
	v.image.File = uri.Path()
	v.image.FillMode = canvas.ImageFillContain
	v.image.Refresh()
}

func (v *Viewer) SetColorIndicators(colors []model.ColorLabel) {
	v.indicators.RemoveAll()
	for _, c := range colors {
		circle := canvas.NewCircle(colorLabelToColor(c))
		circle.Resize(fyne.NewSize(colorIndicatorSize, colorIndicatorSize))
		v.indicators.Add(circle)
	}
	v.indicators.Refresh()
}

func (v *Viewer) Clear() {
	v.image.Resource = nil
	v.image.File = ""
	v.image.Refresh()
	v.indicators.RemoveAll()
}

func (v *Viewer) build() {
	v.image = &canvas.Image{}
	v.image.FillMode = canvas.ImageFillContain

	v.indicators = container.NewHBox()

	indicatorOverlay := container.NewStack(
		v.image,
		container.NewVBox(
			v.indicators,
		),
	)

	v.container = container.NewStack(indicatorOverlay)
}

func colorLabelToColor(label model.ColorLabel) color.Color {
	switch label {
	case model.ColorRed:
		return color.NRGBA{R: 255, G: 0, B: 0, A: 255}
	case model.ColorGreen:
		return color.NRGBA{R: 0, G: 255, B: 0, A: 255}
	case model.ColorBlue:
		return color.NRGBA{R: 0, G: 100, B: 255, A: 255}
	default:
		return color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	}
}
