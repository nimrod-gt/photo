package ui

import (
	"image/color"
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
	gridColumns       = 3
	gridThumbRatio    = 220.0 / 300.0
	gridPreloadBuffer = 50
)

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

	visibleMin int
	visibleMax int

	imageLoader *service.ImageLoader
	callbacks   GridViewerCallbacks
}

func NewGridViewer(callbacks GridViewerCallbacks) *GridViewer {
	gv := &GridViewer{callbacks: callbacks}
	gv.build()
	return gv
}

func (gv *GridViewer) Container() *fyne.Container {
	return gv.container
}

func (gv *GridViewer) SetImageLoader(loader *service.ImageLoader) {
	gv.imageLoader = loader
}

func (gv *GridViewer) StopLoading() {
}

func (gv *GridViewer) SetPhotos(photos []model.Photo, meta []model.PhotoMeta) {
	gv.mu.Lock()
	gv.photos = photos
	gv.meta = meta
	gv.visibleMin = 0
	gv.visibleMax = 0
	gv.mu.Unlock()
	gv.grid.Refresh()
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

	switch {
	case gv.imageLoader != nil:
		if img := gv.imageLoader.Peek(photo.ImagePath, service.SizeThumb); img != nil {
			thumb.Image = img
			thumb.Show()
		} else if meta.Thumbnail != nil {
			thumb.Image = meta.Thumbnail
			thumb.Show()
			gv.schedulePreload()
		} else {
			thumb.Image = nil
			thumb.Hide()
			gv.schedulePreload()
		}
	case meta.Thumbnail != nil:
		thumb.Image = meta.Thumbnail
		thumb.Show()
	default:
		thumb.Image = nil
		thumb.Hide()
	}
	thumb.Refresh()

	nameLabel.SetText(photo.Name)
	updateColorDots(dotsContainer, meta.Colors)

	gv.mu.Lock()
	if id < gv.visibleMin || gv.visibleMin == gv.visibleMax {
		gv.visibleMin = id
	}
	if id > gv.visibleMax || gv.visibleMin == gv.visibleMax {
		gv.visibleMax = id
	}
	gv.mu.Unlock()
}

func (gv *GridViewer) schedulePreload() {
	if gv.imageLoader == nil {
		return
	}

	gv.mu.Lock()
	lo := max(gv.visibleMin-gridPreloadBuffer, 0)
	hi := min(gv.visibleMax+gridPreloadBuffer, len(gv.photos)-1)
	paths := make([]string, 0, hi-lo+1)
	indexByPath := make(map[string]int, hi-lo+1)
	for i := lo; i <= hi; i++ {
		p := gv.photos[i].ImagePath
		paths = append(paths, p)
		indexByPath[p] = i
	}
	gv.mu.Unlock()

	grid := gv.grid
	go gv.imageLoader.Preload(paths, service.SizeThumb, func(path string) {
		gv.mu.Lock()
		idx, ok := indexByPath[path]
		gv.mu.Unlock()
		if ok {
			fyne.Do(func() {
				grid.RefreshItem(idx)
			})
		}
	})
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
