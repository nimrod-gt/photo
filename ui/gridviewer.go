package ui

import (
	"sync"
	"sync/atomic"

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
	container        *fyne.Container
	grid             *widget.GridWrap
	imageProvider    *service.ImageProvider
	callbacks        GridViewerCallbacks
	photos           []model.Photo
	meta             []model.PhotoMeta
	visibleMin       int
	visibleMax       int
	mu               sync.Mutex
	tileWidth        float32
	preloadScheduled atomic.Bool
	items            map[fyne.CanvasObject]*gridItem
}

type gridItem struct {
	thumb *canvas.Image
	name  *widget.Label
}

func NewGridViewer(imageProvider *service.ImageProvider, callbacks GridViewerCallbacks) *GridViewer {
	gv := &GridViewer{
		imageProvider: imageProvider,
		callbacks:     callbacks,
		items:         make(map[fyne.CanvasObject]*gridItem),
	}
	gv.build()
	return gv
}

func (gv *GridViewer) thumbPixelSize() int {
	gv.mu.Lock()
	defer gv.mu.Unlock()
	return max(int(gv.tileWidth*2), 256)
}

func (gv *GridViewer) Container() *fyne.Container {
	return gv.container
}

func (gv *GridViewer) StopLoading() {
	gv.imageProvider.BumpGen()
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

	root := container.NewBorder(nil, nameLabel, nil, nil, thumb)
	gv.items[root] = &gridItem{thumb: thumb, name: nameLabel}
	return root
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

	item, ok := gv.items[obj]
	if !ok {
		return
	}
	thumb := item.thumb
	nameLabel := item.name

	thumbSize := gv.thumbPixelSize()
	if img := gv.imageProvider.Peek(photo.ImagePath, thumbSize); img != nil {
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
	thumb.Refresh()

	nameLabel.SetText(photo.Name)

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
	if !gv.preloadScheduled.CompareAndSwap(false, true) {
		return
	}

	gen := gv.imageProvider.Gen()

	gv.mu.Lock()
	if len(gv.photos) == 0 {
		gv.mu.Unlock()
		gv.preloadScheduled.Store(false)
		return
	}
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
	go func() {
		defer gv.preloadScheduled.Store(false)
		gv.imageProvider.Preload(paths, gv.thumbPixelSize(), func(path string) {
			if gen != gv.imageProvider.Gen() {
				return
			}
			gv.mu.Lock()
			idx, ok := indexByPath[path]
			gv.mu.Unlock()
			if ok {
				fyne.Do(func() {
					grid.RefreshItem(idx)
				})
			}
		})
	}()
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
