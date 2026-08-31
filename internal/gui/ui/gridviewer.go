package ui

import (
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"photo/internal/core/model"
)

const (
	gridColumns       = 3
	gridThumbRatio    = 220.0 / 300.0
	gridPreloadBuffer = 20
	gridPreloadDelay  = 20 * time.Millisecond
	gridRefreshDelay  = 50 * time.Millisecond
)

type gridImageProvider interface {
	thumbnailSource
	Preload(paths []string, size int, onLoaded func(string))
	Gen() uint64
	BumpGen()
}

type GridViewerCallbacks struct {
	OnPhotoTapped func(index int)
}

type GridViewer struct {
	container     *fyne.Container
	grid          *widget.GridWrap
	imageProvider gridImageProvider
	callbacks     GridViewerCallbacks
	photos        []model.Photo
	meta          []model.PhotoMeta
	visible       visibleRange
	mu            sync.Mutex
	tileWidth     float32
	// Provider.Preload only hands the paths to its workers and returns, so a
	// dispatch per miss would fire once per tile of the row being drawn. The
	// misses are collected for one delay instead, which is what lets the visible
	// range grow to cover the whole row before anything is warmed.
	preloadPending atomic.Bool
	// one RefreshItem re-runs updateItem for every visible tile, so a whole
	// batch of loaded photos needs exactly one of them
	refreshPending atomic.Bool
	// time.AfterFunc as a field, so the tests fire the delays themselves: a
	// stalled machine must not be able to split one coalescing window into two
	// dispatches.
	after func(time.Duration, func())
	// GridWrap.RefreshItem does nothing until the grid sits in a scroller, so
	// the call is held as a field the tests can watch.
	refreshItem func(int)
}

func NewGridViewer(imageProvider gridImageProvider, callbacks GridViewerCallbacks) *GridViewer {
	gv := &GridViewer{
		imageProvider: imageProvider,
		callbacks:     callbacks,
		after:         func(delay time.Duration, run func()) { time.AfterFunc(delay, run) },
	}
	gv.build()
	return gv
}

func (gv *GridViewer) thumbPixelSize() int {
	gv.mu.Lock()
	defer gv.mu.Unlock()
	return gv.thumbPixelSizeLocked()
}

func (gv *GridViewer) thumbPixelSizeLocked() int {
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
	gv.visible.reset()
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
	gv.refreshItem = gv.grid.RefreshItem

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
	return newGridItem(tw)
}

func (gv *GridViewer) updateItem(id widget.GridWrapItemID, obj fyne.CanvasObject) {
	gv.mu.Lock()
	if id >= len(gv.photos) || id >= len(gv.meta) {
		gv.mu.Unlock()
		return
	}
	photo := gv.photos[id]
	meta := gv.meta[id]
	// before the preload below, so the dispatch it may trigger already knows
	// which tile asked for it
	gv.visible.observe(id)
	size := gv.thumbPixelSizeLocked()
	gv.mu.Unlock()

	item, ok := obj.(*gridItem)
	if !ok {
		return
	}

	thumbnail, needsLoad := resolveThumbnail(gv.imageProvider, photo.ImagePath, meta.Thumbnail, size)
	if needsLoad {
		gv.schedulePreload()
	}
	item.update(photo.Name, thumbnail)
}

func (gv *GridViewer) schedulePreload() {
	if !gv.preloadPending.CompareAndSwap(false, true) {
		return
	}
	gv.after(gridPreloadDelay, func() {
		// cleared before the window is taken, so a tile missing from here on
		// arms the next dispatch instead of being dropped
		gv.preloadPending.Store(false)
		gv.dispatchPreload()
	})
}

func (gv *GridViewer) dispatchPreload() {
	gen := gv.imageProvider.Gen()

	gv.mu.Lock()
	lo, hi := gv.visible.bounds(len(gv.photos), gridPreloadBuffer)
	gv.visible.reset()
	// snapshotted here rather than read in the goroutine: a resize in between
	// would cache the images at a size the tile's own Peek can never satisfy
	size := gv.thumbPixelSizeLocked()
	count := max(hi-lo+1, 0)
	paths := make([]string, 0, count)
	indexByPath := make(map[string]int, count)
	for i := lo; i <= hi; i++ {
		p := gv.photos[i].ImagePath
		paths = append(paths, p)
		indexByPath[p] = i
	}
	gv.mu.Unlock()

	if len(paths) == 0 {
		return
	}

	go func() {
		gv.imageProvider.Preload(paths, size, func(path string) {
			if gen != gv.imageProvider.Gen() {
				return
			}
			gv.mu.Lock()
			idx, ok := indexByPath[path]
			gv.mu.Unlock()
			if ok {
				gv.scheduleRefresh(idx)
			}
		})
	}()
}

func (gv *GridViewer) scheduleRefresh(index int) {
	if !gv.refreshPending.CompareAndSwap(false, true) {
		return
	}
	gv.after(gridRefreshDelay, func() {
		gv.refreshPending.Store(false)
		fyne.Do(func() {
			gv.refreshItem(index)
		})
	})
}

type gridResizeLayout struct {
	zeroMinSize
	onResize func(fyne.Size)
	lastSize fyne.Size
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
