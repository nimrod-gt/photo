package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"photo/model"
)

type FileBrowserCallbacks struct {
	OnPhotoSelected func(photo model.Photo)
}

type FileBrowser struct {
	container *fyne.Container
	list      *widget.List
	photos    []model.Photo
	callbacks FileBrowserCallbacks
}

func NewFileBrowser(callbacks FileBrowserCallbacks) *FileBrowser {
	fb := &FileBrowser{callbacks: callbacks}
	fb.build()
	return fb
}

func (fb *FileBrowser) Container() *fyne.Container {
	return fb.container
}

func (fb *FileBrowser) SetPhotos(photos []model.Photo) {
	fb.photos = photos
	fb.list.Refresh()
}

func (fb *FileBrowser) SelectIndex(index int) {
	if index >= 0 && index < len(fb.photos) {
		fb.list.Select(index)
	}
}

func (fb *FileBrowser) build() {
	fb.list = widget.NewList(
		func() int {
			return len(fb.photos)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("placeholder")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < len(fb.photos) {
				obj.(*widget.Label).SetText(fb.photos[id].Name)
			}
		},
	)

	fb.list.OnSelected = func(id widget.ListItemID) {
		if id < len(fb.photos) && fb.callbacks.OnPhotoSelected != nil {
			fb.callbacks.OnPhotoSelected(fb.photos[id])
		}
	}

	fb.container = container.NewStack(fb.list)
}
