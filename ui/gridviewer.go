package ui

import (
	"context"
	"image"
	"image/color"
	"runtime"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"photo/model"
	"photo/service"
)

const (
	gridColumns    = 3
	gridThumbRatio = 220.0 / 300.0
)

var gridLoadWorkers = max(runtime.NumCPU()-1, 2)

type GridViewerCallbacks struct {
	OnPhotoTapped func(index int)
}

type GridViewer struct {
	container *fyne.Container
	grid      *widget.GridWrap

	mu        sync.Mutex
	photos    []model.Photo
	meta      []model.PhotoMeta
	tileWidth float32
	targetW   int

	loader     func(path string) (image.Image, error)
	cancelLoad context.CancelFunc
	jobs       chan gridLoadJob

	callbacks GridViewerCallbacks
}

func NewGridViewer(callbacks GridViewerCallbacks) *GridViewer {
	gv := &GridViewer{callbacks: callbacks}
	gv.build()
	return gv
}

func (gv *GridViewer) Container() *fyne.Container {
	return gv.container
}

func (gv *GridViewer) SetLoader(fn func(string) (image.Image, error)) {
	gv.loader = fn
}

func (gv *GridViewer) StopLoading() {
	gv.mu.Lock()
	cancel := gv.cancelLoad
	gv.cancelLoad = nil
	gv.jobs = nil
	gv.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (gv *GridViewer) SetPhotos(photos []model.Photo, meta []model.PhotoMeta) {
	gv.StopLoading()

	gv.mu.Lock()
	gv.photos = photos
	gv.meta = meta
	tileWidth := gv.tileWidth
	gv.mu.Unlock()
	gv.grid.Refresh()

	if gv.loader == nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	gv.mu.Lock()
	gv.cancelLoad = cancel
	gv.mu.Unlock()

	scale := gv.canvasScale()
	targetW := int(tileWidth * scale)
	targetH := int(tileWidth * gridThumbRatio * scale)
	maxSize := image.Point{X: targetW, Y: targetH}

	jobs := make(chan gridLoadJob, 50)

	gv.mu.Lock()
	gv.targetW = targetW
	gv.jobs = jobs
	gv.mu.Unlock()

	for range gridLoadWorkers {
		go func() {
			for {
				select {
				case job := <-jobs:
					gv.loadSingleThumbnail(ctx, job, maxSize)
				case <-ctx.Done():
					return
				}
			}
		}()
	}
}

type gridLoadJob struct {
	index int
	photo model.Photo
}

func (gv *GridViewer) loadSingleThumbnail(ctx context.Context, job gridLoadJob, maxSize image.Point) {
	img, err := gv.loader(job.photo.ImagePath)
	if err != nil || ctx.Err() != nil {
		return
	}

	scaled := service.DownscaleToFit(img, maxSize)

	gv.mu.Lock()
	if job.index < len(gv.meta) && ctx.Err() == nil {
		gv.meta[job.index].Thumbnail = scaled
	}
	gv.mu.Unlock()

	if ctx.Err() == nil {
		fyne.Do(func() {
			gv.grid.RefreshItem(job.index)
		})
	}
}

func (gv *GridViewer) canvasScale() float32 {
	c := fyne.CurrentApp().Driver().CanvasForObject(gv.grid)
	if c == nil {
		return 2
	}
	s := c.Scale()
	if s < 1 {
		return 2
	}
	return s
}

func (gv *GridViewer) ScrollToIndex(index int) {
	gv.grid.ScrollTo(index)
}

func (gv *GridViewer) build() {
	gv.tileWidth = 200

	gv.grid = widget.NewGridWrap(
		func() int {
			gv.mu.Lock()
			defer gv.mu.Unlock()
			return len(gv.photos)
		},
		gv.createItem,
		gv.updateItem,
	)

	gv.grid.OnSelected = func(id widget.GridWrapItemID) {
		gv.grid.UnselectAll()
		if gv.callbacks.OnPhotoTapped != nil {
			gv.callbacks.OnPhotoTapped(id)
		}
	}

	resizeLayout := &gridResizeLayout{
		onResize: func(size fyne.Size) {
			gv.recalcTileSize(size.Width)
		},
	}
	gv.container = container.New(resizeLayout, gv.grid)
}

func (gv *GridViewer) recalcTileSize(availableWidth float32) {
	padding := theme.Padding()
	newWidth := (availableWidth+padding)/float32(gridColumns) - padding - 1
	if newWidth < 100 {
		newWidth = 100
	}
	gv.mu.Lock()
	changed := gv.tileWidth != newWidth
	gv.tileWidth = newWidth
	gv.mu.Unlock()
	if changed {
		gv.grid.Refresh()
	}
}

func (gv *GridViewer) createItem() fyne.CanvasObject {
	gv.mu.Lock()
	tw := gv.tileWidth
	gv.mu.Unlock()
	thumbH := tw * gridThumbRatio
	thumb := canvas.NewImageFromImage(nil)
	thumb.SetMinSize(fyne.NewSize(tw, thumbH))
	thumb.FillMode = canvas.ImageFillContain

	nameLabel := widget.NewLabel("placeholder")
	nameLabel.Truncation = fyne.TextTruncateEllipsis
	nameLabel.Alignment = fyne.TextAlignCenter

	dot1 := canvas.NewText("\u25CF", color.Transparent)
	dot1.TextSize = 10
	dot1.Hide()
	dot2 := canvas.NewText("\u25CF", color.Transparent)
	dot2.TextSize = 10
	dot2.Hide()
	dot3 := canvas.NewText("\u25CF", color.Transparent)
	dot3.TextSize = 10
	dot3.Hide()
	dotsContainer := container.NewHBox(dot1, dot2, dot3)

	nameRow := container.NewBorder(nil, nil, dotsContainer, nil, nameLabel)

	return container.NewBorder(nil, nameRow, nil, nil, thumb)
}

func (gv *GridViewer) updateItem(id widget.GridWrapItemID, obj fyne.CanvasObject) {
	gv.mu.Lock()
	if id >= len(gv.photos) || id >= len(gv.meta) {
		gv.mu.Unlock()
		return
	}
	photo := gv.photos[id]
	meta := gv.meta[id]
	gv.mu.Unlock()

	tile := obj.(*fyne.Container)
	thumb := tile.Objects[0].(*canvas.Image)
	nameRow := tile.Objects[1].(*fyne.Container)
	nameLabel := nameRow.Objects[0].(*widget.Label)
	dotsContainer := nameRow.Objects[1].(*fyne.Container)

	if meta.Thumbnail != nil {
		thumb.Image = meta.Thumbnail
		thumb.Show()
	} else {
		thumb.Image = nil
		thumb.Hide()
	}
	thumb.Refresh()

	nameLabel.SetText(photo.Name)
	updateColorDots(dotsContainer, meta.Colors)

	gv.mu.Lock()
	targetW := gv.targetW
	jobs := gv.jobs
	gv.mu.Unlock()

	if jobs != nil && (meta.Thumbnail == nil || meta.Thumbnail.Bounds().Dx() < targetW) {
		select {
		case jobs <- gridLoadJob{index: id, photo: photo}:
		default:
		}
	}
}

type gridResizeLayout struct {
	onResize func(fyne.Size)
	lastSize fyne.Size
}

func (l *gridResizeLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, 0)
}

func (l *gridResizeLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if size != l.lastSize {
		l.lastSize = size
		if l.onResize != nil {
			l.onResize(size)
		}
	}
	for _, obj := range objects {
		obj.Resize(size)
		obj.Move(fyne.NewPos(0, 0))
	}
}
