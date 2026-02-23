package ui

import (
	"image"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

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

func (v *Viewer) ShowPhoto(img image.Image) {
	v.image.Resource = nil
	v.image.File = ""
	v.image.Image = img
	v.image.FillMode = canvas.ImageFillContain
	v.image.Show()
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
	v.image.Hide()
	v.indicators.RemoveAll()
}

func (v *Viewer) build() {
	v.image = &canvas.Image{}
	v.image.FillMode = canvas.ImageFillContain

	v.indicators = container.NewHBox()

	overlay := newTappableOverlay(v.callbacks)

	indicatorOverlay := container.NewStack(
		v.image,
		container.NewVBox(
			v.indicators,
		),
		overlay,
	)

	v.container = container.NewStack(indicatorOverlay)
}

type tappableOverlay struct {
	widget.BaseWidget
	callbacks ViewerCallbacks
}

func newTappableOverlay(callbacks ViewerCallbacks) *tappableOverlay {
	o := &tappableOverlay{callbacks: callbacks}
	o.ExtendBaseWidget(o)
	return o
}

func (o *tappableOverlay) CreateRenderer() fyne.WidgetRenderer {
	return &tappableOverlayRenderer{}
}

func (o *tappableOverlay) Tapped(_ *fyne.PointEvent) {
	if o.callbacks.OnTapped != nil {
		o.callbacks.OnTapped()
	}
}

func (o *tappableOverlay) TappedSecondary(ev *fyne.PointEvent) {
	if o.callbacks.OnSecondaryTapped != nil {
		o.callbacks.OnSecondaryTapped(ev.AbsolutePosition)
	}
}

type tappableOverlayRenderer struct{}

func (r *tappableOverlayRenderer) Destroy()                     {}
func (r *tappableOverlayRenderer) Layout(_ fyne.Size)           {}
func (r *tappableOverlayRenderer) MinSize() fyne.Size           { return fyne.NewSize(0, 0) }
func (r *tappableOverlayRenderer) Objects() []fyne.CanvasObject { return nil }
func (r *tappableOverlayRenderer) Refresh()                     {}
