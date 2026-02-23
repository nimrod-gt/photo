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
	dateFontSize    float32 = 11
)

type FileBrowserCallbacks struct {
	OnPhotoSelected     func(photo model.Photo)
	OnDirectorySelected func(dir string)
	OnChooseFolder      func()
	OnSortBy            func(order service.SortOrder)
}

type FileBrowser struct {
	container   *fyne.Container
	list        *widget.List
	nameSortBtn *widget.Button
	timeSortBtn *widget.Button
	dirTree     *DirTree
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

func (fb *FileBrowser) SetSortState(order service.SortOrder, descending bool) {
	arrow := " \u2191"
	if descending {
		arrow = " \u2193"
	}
	if order == service.SortByName {
		fb.nameSortBtn.SetText("Name" + arrow)
		fb.nameSortBtn.Importance = widget.HighImportance
		fb.timeSortBtn.SetText("Time")
		fb.timeSortBtn.Importance = widget.MediumImportance
	} else {
		fb.nameSortBtn.SetText("Name")
		fb.nameSortBtn.Importance = widget.MediumImportance
		fb.timeSortBtn.SetText("Time" + arrow)
		fb.timeSortBtn.Importance = widget.HighImportance
	}
	fb.nameSortBtn.Refresh()
	fb.timeSortBtn.Refresh()
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

	fb.nameSortBtn = widget.NewButton("Name \u2191", func() {
		if fb.callbacks.OnSortBy != nil {
			fb.callbacks.OnSortBy(service.SortByName)
		}
	})
	fb.nameSortBtn.Importance = widget.HighImportance
	fb.timeSortBtn = widget.NewButton("Time", func() {
		if fb.callbacks.OnSortBy != nil {
			fb.callbacks.OnSortBy(service.SortByTime)
		}
	})
	fb.timeSortBtn.Importance = widget.MediumImportance
	sortBar := container.NewGridWithColumns(2, fb.nameSortBtn, fb.timeSortBtn)

	treeWithBtn := container.NewBorder(chooseBtn, nil, nil, nil, fb.dirTree.Widget())
	listWithSort := container.NewBorder(sortBar, nil, nil, nil, fb.list)
	split := container.NewVSplit(treeWithBtn, listWithSort)
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

	dot1 := canvas.NewText("\u25CF", color.Transparent)
	dot1.TextSize = 12
	dot1.Hide()
	dot2 := canvas.NewText("\u25CF", color.Transparent)
	dot2.TextSize = 12
	dot2.Hide()
	dot3 := canvas.NewText("\u25CF", color.Transparent)
	dot3.TextSize = 12
	dot3.Hide()

	dotsContainer := container.NewHBox(dot1, dot2, dot3)

	topRow := container.NewBorder(nil, nil, nil, star, nameLabel)

	dateText := canvas.NewText("", color.NRGBA{R: 160, G: 160, B: 160, A: 255})
	dateText.TextSize = dateFontSize
	dateContent := container.New(layout.NewCustomPaddedLayout(0, 0, theme.InnerPadding(), 0), dateText)
	dateRow := container.NewBorder(nil, nil, nil, dotsContainer, dateContent)

	textBox := container.NewVBox(topRow, dateRow)

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
	dateRow := textBox.Objects[1].(*fyne.Container)
	dateContent := dateRow.Objects[0].(*fyne.Container)
	dateText := dateContent.Objects[0].(*canvas.Text)

	updateThumbnail(thumb, meta.Thumbnail)
	topRow.Objects[0].(*widget.Label).SetText(photo.Name)
	updateStar(topRow.Objects[1].(*canvas.Text), meta.Favorite)
	updateColorDots(dateRow.Objects[1].(*fyne.Container), meta.Colors)
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
		obj.(*canvas.Text).Hide()
	}
	has := ColorSet(colors)
	idx := 0
	for _, c := range colorOrder {
		if !has[c] || idx >= len(dotsContainer.Objects) {
			continue
		}
		dot := dotsContainer.Objects[idx].(*canvas.Text)
		dot.Color = colorLabelToColor(c)
		dot.Show()
		dot.Refresh()
		idx++
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
