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
)

const (
	gridColumns        = 3
	gridThumbRatio     = 220.0 / 300.0
	gridNameRowHeight  = 40
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

func (gv *GridViewer) SetPhotos(photos []model.Photo, meta []model.PhotoMeta) {
	gv.mu.Lock()
	gv.photos = photos
	gv.meta = meta
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
	if gv.tileWidth != newWidth {
		gv.tileWidth = newWidth
		gv.grid.Refresh()
	}
}

func (gv *GridViewer) createItem() fyne.CanvasObject {
	thumbH := gv.tileWidth * gridThumbRatio
	thumb := canvas.NewImageFromImage(nil)
	thumb.SetMinSize(fyne.NewSize(gv.tileWidth, thumbH))
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
	if id >= len(gv.photos) {
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
