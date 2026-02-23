package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"photo/model"
	"photo/service"
)

type FileBrowserCallbacks struct {
	OnPhotoSelected     func(photo model.Photo)
	OnDirectorySelected func(dir string)
	OnChooseFolder      func()
}

type FileBrowser struct {
	container *fyne.Container
	list      *widget.List
	dirTree   *DirTree
	photos    []model.Photo
	callbacks FileBrowserCallbacks
}

func NewFileBrowser(scanner *service.Scanner, callbacks FileBrowserCallbacks) *FileBrowser {
	fb := &FileBrowser{callbacks: callbacks}
	fb.dirTree = NewDirTree(scanner, callbacks.OnDirectorySelected)
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

func (fb *FileBrowser) SetRoot(root string) {
	fb.dirTree.SetRoot(root)
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

	chooseBtn := widget.NewButton("Open Folder...", func() {
		if fb.callbacks.OnChooseFolder != nil {
			fb.callbacks.OnChooseFolder()
		}
	})

	treeWithBtn := container.NewBorder(chooseBtn, nil, nil, nil, fb.dirTree.Widget())
	split := container.NewVSplit(treeWithBtn, fb.list)
	split.SetOffset(0.4)

	fb.container = container.NewStack(split)
}
