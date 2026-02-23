package ui

import (
	"context"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"photo/model"
	"photo/service"
)

const (
	thumbnailWidth  float32 = 48
	thumbnailHeight float32 = 36
	colorDotSize    float32 = 8
	dateFontSize    float32 = 11
)

type FileBrowserCallbacks struct {
	OnPhotoSelected     func(photo model.Photo)
	OnDirectorySelected func(dir string)
	OnChooseFolder      func()
}

type FileBrowser struct {
	container  *fyne.Container
	list       *widget.List
	dirTree    *DirTree
	photos     []model.Photo
	meta       []model.PhotoMeta
	generation uint64
	mu         sync.Mutex
	loader     *service.MetadataLoader
	colors     *service.ColorService
	cancel     context.CancelFunc
	callbacks  FileBrowserCallbacks
}

func NewFileBrowser(
	scanner *service.Scanner,
	loader *service.MetadataLoader,
	colors *service.ColorService,
	callbacks FileBrowserCallbacks,
) *FileBrowser {
	fb := &FileBrowser{
		loader:    loader,
		colors:    colors,
		callbacks: callbacks,
		cancel:    func() {},
	}
	fb.dirTree = NewDirTree(scanner, callbacks.OnDirectorySelected)
	fb.build()
	return fb
}

func (fb *FileBrowser) Container() *fyne.Container {
	return fb.container
}

func (fb *FileBrowser) SetPhotos(photos []model.Photo) {
	fb.mu.Lock()
	fb.cancel()
	fb.photos = photos
	fb.meta = make([]model.PhotoMeta, len(photos))
	fb.generation++
	gen := fb.generation
	fb.mu.Unlock()

	fb.loadInitialMeta(photos)

	fb.list.Refresh()

	ctx, cancel := context.WithCancel(context.Background())
	fb.mu.Lock()
	fb.cancel = cancel
	fb.mu.Unlock()
	fb.loader.LoadAsync(ctx, photos, func(index int, thumbnail image.Image, favorite bool) {
		fb.mu.Lock()
		if fb.generation != gen {
			fb.mu.Unlock()
			return
		}
		if index < len(fb.meta) {
			fb.meta[index].Thumbnail = thumbnail
			fb.meta[index].Favorite = favorite
		}
		fb.mu.Unlock()
		fyne.Do(func() {
			fb.list.RefreshItem(index)
		})
	})
}

func (fb *FileBrowser) loadInitialMeta(photos []model.Photo) {
	if len(photos) == 0 {
		return
	}

	dir := filepath.Dir(photos[0].ImagePath)
	colorMap, _ := fb.colors.GetDirectoryColors(dir)

	dates := make([]time.Time, len(photos))
	for i, photo := range photos {
		if info, err := os.Stat(photo.ImagePath); err == nil {
			dates[i] = info.ModTime()
		}
	}

	fb.mu.Lock()
	defer fb.mu.Unlock()

	for i, photo := range photos {
		if colorMap != nil {
			fb.meta[i].Colors = colorMap[photo.Name]
		}
		fb.meta[i].Date = dates[i]
	}
}

func (fb *FileBrowser) RefreshItemMeta(index int, colors []model.ColorLabel, favorite bool) {
	fb.mu.Lock()
	if index >= 0 && index < len(fb.meta) {
		fb.meta[index].Colors = colors
		fb.meta[index].Favorite = favorite
	}
	fb.mu.Unlock()
	fb.list.RefreshItem(index)
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
			fb.mu.Lock()
			defer fb.mu.Unlock()
			return len(fb.photos)
		},
		fb.createItem,
		fb.updateItem,
	)

	fb.list.OnSelected = func(id widget.ListItemID) {
		fb.mu.Lock()
		inRange := id < len(fb.photos)
		var photo model.Photo
		if inRange {
			photo = fb.photos[id]
		}
		fb.mu.Unlock()

		if inRange && fb.callbacks.OnPhotoSelected != nil {
			fb.callbacks.OnPhotoSelected(photo)
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

func (fb *FileBrowser) createItem() fyne.CanvasObject {
	thumb := canvas.NewImageFromImage(nil)
	thumb.SetMinSize(fyne.NewSize(thumbnailWidth, thumbnailHeight))
	thumb.FillMode = canvas.ImageFillContain

	nameLabel := widget.NewLabel("placeholder")
	nameLabel.Truncation = fyne.TextTruncateEllipsis

	star := canvas.NewText("\u2605", color.NRGBA{R: 255, G: 215, B: 0, A: 255})
	star.TextSize = 14
	star.Hide()

	dot1 := canvas.NewCircle(color.Transparent)
	dot1.Resize(fyne.NewSize(colorDotSize, colorDotSize))
	dot1.Hide()
	dot2 := canvas.NewCircle(color.Transparent)
	dot2.Resize(fyne.NewSize(colorDotSize, colorDotSize))
	dot2.Hide()
	dot3 := canvas.NewCircle(color.Transparent)
	dot3.Resize(fyne.NewSize(colorDotSize, colorDotSize))
	dot3.Hide()

	dotsContainer := container.NewHBox(dot1, dot2, dot3)

	indicators := container.NewHBox(star, dotsContainer)

	topRow := container.NewBorder(nil, nil, nil, indicators, nameLabel)

	dateText := canvas.NewText("", color.NRGBA{R: 160, G: 160, B: 160, A: 255})
	dateText.TextSize = dateFontSize
	datePadded := container.New(layout.NewCustomPaddedLayout(0, 0, theme.InnerPadding(), 0), dateText)

	textBox := container.NewVBox(topRow, datePadded)

	return container.NewBorder(nil, nil, thumb, nil,
		container.New(layout.NewStackLayout(), textBox),
	)
}

func (fb *FileBrowser) updateItem(id widget.ListItemID, obj fyne.CanvasObject) {
	fb.mu.Lock()
	if id >= len(fb.photos) {
		fb.mu.Unlock()
		return
	}
	photo := fb.photos[id]
	meta := fb.meta[id]
	fb.mu.Unlock()

	root := obj.(*fyne.Container)
	thumb := root.Objects[1].(*canvas.Image)
	textStack := root.Objects[0].(*fyne.Container)
	textBox := textStack.Objects[0].(*fyne.Container)
	topRow := textBox.Objects[0].(*fyne.Container)
	datePadded := textBox.Objects[1].(*fyne.Container)
	dateText := datePadded.Objects[0].(*canvas.Text)
	indicators := topRow.Objects[1].(*fyne.Container)

	updateThumbnail(thumb, meta.Thumbnail)
	topRow.Objects[0].(*widget.Label).SetText(photo.Name)
	updateStar(indicators.Objects[0].(*canvas.Text), meta.Favorite)
	updateColorDots(indicators.Objects[1].(*fyne.Container), meta.Colors)
	updateDateText(dateText, meta.Date)
}

func updateThumbnail(thumb *canvas.Image, img image.Image) {
	if img != nil {
		thumb.Image = img
		thumb.Show()
	} else {
		thumb.Image = nil
		thumb.Hide()
	}
	thumb.Refresh()
}

func updateStar(star *canvas.Text, favorite bool) {
	if favorite {
		star.Show()
	} else {
		star.Hide()
	}
}

func updateColorDots(dotsContainer *fyne.Container, colors []model.ColorLabel) {
	for _, obj := range dotsContainer.Objects {
		obj.(*canvas.Circle).Hide()
	}
	for i, c := range colors {
		if i >= len(dotsContainer.Objects) {
			break
		}
		dot := dotsContainer.Objects[i].(*canvas.Circle)
		dot.FillColor = colorLabelToColor(c)
		dot.Show()
		dot.Refresh()
	}
}

func updateDateText(dateText *canvas.Text, date time.Time) {
	if !date.IsZero() {
		dateText.Text = date.Format("2006-01-02 15:04")
	} else {
		dateText.Text = ""
	}
	dateText.Refresh()
}
