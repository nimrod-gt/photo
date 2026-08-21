package ui

import (
	"image"
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"photo/internal/core/model"
)

const (
	indicatorTextSize = 18
	minZoom           = 1.0
	maxZoom           = 10.0
	keyboardZoomStep  = 1.3
)

type ViewerCallbacks struct {
	OnTapped          func()
	OnSecondaryTapped func(pos fyne.Position)
	OnZoomChanged     func(zoom float32)
}

type Viewer struct {
	container  *fyne.Container
	zoomW      *zoomWidget
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
	v.zoomW.setImage(img)
	v.resetZoom()
}

func (v *Viewer) UpdateImage(img image.Image) {
	v.zoomW.setImage(img)
	v.zoomW.Refresh()
}

// The marks are text rather than canvas circles: the box layout sizes every
// child to its minimum, which for a circle is a single pixel.
func (v *Viewer) SetIndicators(favorite bool, colors []model.ColorLabel) {
	v.indicators.RemoveAll()
	if favorite {
		v.indicators.Add(indicatorText(favoriteMark, favoriteColor))
	}
	has := ColorSet(colors)
	for _, c := range colorOrder {
		if has[c] {
			v.indicators.Add(indicatorText(colorMark, colorLabelToColor(c)))
		}
	}
	v.indicators.Refresh()
}

func indicatorText(mark string, fill color.Color) *canvas.Text {
	text := canvas.NewText(mark, fill)
	text.TextSize = indicatorTextSize
	return text
}

func (v *Viewer) Clear() {
	v.zoomW.image.Hide()
	v.indicators.RemoveAll()
	v.resetZoom()
}

func (v *Viewer) ResetZoom() {
	v.resetZoom()
}

func (v *Viewer) ZoomIn() {
	v.zoomW.zoomBy(keyboardZoomStep, v.zoomW.Size().Width/2, v.zoomW.Size().Height/2)
}

func (v *Viewer) ZoomOut() {
	v.zoomW.zoomBy(1/keyboardZoomStep, v.zoomW.Size().Width/2, v.zoomW.Size().Height/2)
}

func (v *Viewer) resetZoom() {
	v.zoomW.reset()
}

func (v *Viewer) build() {
	v.indicators = container.NewHBox()
	v.zoomW = newZoomWidget(v.callbacks, v.indicators)
	v.container = container.NewStack(v.zoomW)
}

type zoomWidget struct {
	widget.BaseWidget
	image      *canvas.Image
	indicators *fyne.Container
	zoom       float32
	panX       float32
	panY       float32
	callbacks  ViewerCallbacks
}

func newZoomWidget(callbacks ViewerCallbacks, indicators *fyne.Container) *zoomWidget {
	img := &canvas.Image{}
	img.FillMode = canvas.ImageFillStretch
	z := &zoomWidget{
		image:      img,
		indicators: indicators,
		zoom:       minZoom,
		callbacks:  callbacks,
	}
	z.ExtendBaseWidget(z)
	return z
}

func (z *zoomWidget) CreateRenderer() fyne.WidgetRenderer {
	indicatorWrap := container.NewVBox(z.indicators)
	return &zoomRenderer{z: z, objects: []fyne.CanvasObject{z.image, indicatorWrap}}
}

func (z *zoomWidget) setImage(img image.Image) {
	if z.image.Image == img {
		return
	}
	z.image.Resource = nil
	z.image.File = ""
	z.image.Image = img
	z.image.Show()
	z.image.Refresh()
}

func (z *zoomWidget) Scrolled(ev *fyne.ScrollEvent) {
	factor := float32(math.Pow(1.01, float64(ev.Scrolled.DY)))
	z.zoomBy(factor, ev.Position.X, ev.Position.Y)
}

func (z *zoomWidget) zoomBy(factor, cursorX, cursorY float32) {
	newZoom := z.zoom * factor
	if newZoom < minZoom {
		newZoom = minZoom
	}
	if newZoom > maxZoom {
		newZoom = maxZoom
	}
	if newZoom == z.zoom {
		return
	}

	oldZoom := z.zoom
	s := newZoom / oldZoom
	size := z.Size()
	z.panX = z.panX*s + (cursorX-size.Width/2)*(1-s)
	z.panY = z.panY*s + (cursorY-size.Height/2)*(1-s)
	z.zoom = newZoom
	z.clampPan()
	z.Refresh()
	if z.callbacks.OnZoomChanged != nil && newZoom > oldZoom {
		z.callbacks.OnZoomChanged(z.zoom)
	}
}

func (z *zoomWidget) Dragged(ev *fyne.DragEvent) {
	if z.zoom <= minZoom {
		return
	}
	z.panX += ev.Dragged.DX
	z.panY += ev.Dragged.DY
	z.clampPan()
	z.Refresh()
}

func (z *zoomWidget) DragEnd() {}

func (z *zoomWidget) Tapped(_ *fyne.PointEvent) {
	if z.zoom > minZoom {
		z.reset()
		return
	}
	if z.callbacks.OnTapped != nil {
		z.callbacks.OnTapped()
	}
}

func (z *zoomWidget) reset() {
	z.zoom = minZoom
	z.panX = 0
	z.panY = 0
	z.Refresh()
}

func (z *zoomWidget) TappedSecondary(ev *fyne.PointEvent) {
	if z.callbacks.OnSecondaryTapped != nil {
		z.callbacks.OnSecondaryTapped(ev.AbsolutePosition)
	}
}

func (z *zoomWidget) clampPan() {
	size := z.Size()
	fitted := z.fittedSize(size)
	dispW := fitted.Width * z.zoom
	dispH := fitted.Height * z.zoom

	maxPanX := max(0, (dispW-size.Width)/2)
	maxPanY := max(0, (dispH-size.Height)/2)

	z.panX = max(-maxPanX, min(z.panX, maxPanX))
	z.panY = max(-maxPanY, min(z.panY, maxPanY))
}

func (z *zoomWidget) fittedSize(containerSize fyne.Size) fyne.Size {
	if z.image.Image == nil {
		return fyne.NewSize(0, 0)
	}
	b := z.image.Image.Bounds()
	imgW := float32(b.Dx())
	imgH := float32(b.Dy())
	if imgW == 0 || imgH == 0 {
		return fyne.NewSize(0, 0)
	}
	scale := min(containerSize.Width/imgW, containerSize.Height/imgH)
	return fyne.NewSize(imgW*scale, imgH*scale)
}

type zoomRenderer struct {
	z       *zoomWidget
	objects []fyne.CanvasObject
}

func (r *zoomRenderer) Layout(size fyne.Size) {
	fitted := r.z.fittedSize(size)
	if fitted.Width == 0 || fitted.Height == 0 {
		return
	}

	dispW := fitted.Width * r.z.zoom
	dispH := fitted.Height * r.z.zoom

	x := (size.Width-dispW)/2 + r.z.panX
	y := (size.Height-dispH)/2 + r.z.panY

	r.z.image.Resize(fyne.NewSize(dispW, dispH))
	r.z.image.Move(fyne.NewPos(x, y))

	if len(r.objects) > 1 {
		indicators := r.objects[1]
		indicators.Move(fyne.NewPos(0, 0))
		indicators.Resize(indicators.MinSize())
	}
}

func (r *zoomRenderer) MinSize() fyne.Size {
	return fyne.NewSize(0, 0)
}

func (r *zoomRenderer) Refresh() {
	r.Layout(r.z.Size())
}

func (r *zoomRenderer) Destroy() {}

func (r *zoomRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}
