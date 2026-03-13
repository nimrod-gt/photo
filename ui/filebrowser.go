package ui

import (
	"context"
	"image"
	"image/color"
	"log"
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
	OnFilterRed         func()
	OnFilterGreen       func()
	OnFilterBlue        func()
	OnFilterFavorite    func()
	OnFilteredChanged   func(photos []model.Photo)
	OnDeleteAll         func()
	OnCopyAll           func()
}

type FileBrowser struct {
	container   *fyne.Container
	list        *widget.List
	nameSortBtn *widget.Button
	timeSortBtn *widget.Button
	dirTree     *DirTree

	filterFavBtn   *widget.Button
	filterRedBtn   *widget.Button
	filterGreenBtn *widget.Button
	filterBlueBtn  *widget.Button

	bulkBar      *fyne.Container
	deleteAllBtn *widget.Button
	copyAllBtn   *widget.Button

	allPhotos []model.Photo
	allMeta   []model.PhotoMeta
	photos    []model.Photo
	meta      []model.PhotoMeta
	indices   []int

	filterColors   map[model.ColorLabel]bool
	filterFavorite bool
	pinnedPath     string

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
		loader:       loader,
		colors:       colors,
		callbacks:    callbacks,
		cancel:       func() {},
		filterColors: make(map[model.ColorLabel]bool),
	}
	fb.dirTree = NewDirTree(scanner, callbacks.OnDirectorySelected)
	fb.build()
	return fb
}

func (fb *FileBrowser) Container() *fyne.Container {
	return fb.container
}

func (fb *FileBrowser) SetPinnedPath(path string) {
	fb.mu.Lock()
	fb.pinnedPath = path
	fb.mu.Unlock()
}

func (fb *FileBrowser) ClearPinnedPath() {
	fb.mu.Lock()
	fb.pinnedPath = ""
	fb.mu.Unlock()
}

func (fb *FileBrowser) PinnedPath() string {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	return fb.pinnedPath
}

func (fb *FileBrowser) SetPhotos(photos []model.Photo) {
	fb.mu.Lock()
	fb.cancel()
	fb.allPhotos = photos
	fb.allMeta = make([]model.PhotoMeta, len(photos))
	fb.pinnedPath = ""
	fb.generation++
	gen := fb.generation
	fb.mu.Unlock()

	fb.loadInitialMeta(photos)
	fb.applyFilter()
	fb.updateBulkBarVisibility()
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
		if index < len(fb.allMeta) {
			fb.allMeta[index].Thumbnail = thumbnail
			fb.allMeta[index].Favorite = favorite
		}
		displayIdx := fb.displayIndex(index)
		fb.mu.Unlock()
		if displayIdx >= 0 {
			fyne.Do(func() {
				fb.list.RefreshItem(displayIdx)
			})
		}
	}, func() {
		fb.mu.Lock()
		if fb.generation != gen {
			fb.mu.Unlock()
			return
		}
		needRefilter := fb.filterFavorite
		fb.mu.Unlock()
		if needRefilter {
			fyne.Do(func() {
				fb.applyFilter()
				fb.list.Refresh()
				if fb.callbacks.OnFilteredChanged != nil {
					fb.callbacks.OnFilteredChanged(fb.FilteredPhotos())
				}
			})
		}
	})
}

func (fb *FileBrowser) loadInitialMeta(photos []model.Photo) {
	if len(photos) == 0 {
		return
	}

	dir := filepath.Dir(photos[0].ImagePath)
	colorMap, err := fb.colors.GetDirectoryColors(dir)
	if err != nil {
		log.Printf("Failed to load color labels for %s: %v", dir, err)
	}

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
			fb.allMeta[i].Colors = colorMap[photo.Name]
		}
		fb.allMeta[i].Date = dates[i]
	}
}

func (fb *FileBrowser) GetMeta(displayIndex int) model.PhotoMeta {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	if displayIndex >= 0 && displayIndex < len(fb.meta) {
		return fb.meta[displayIndex]
	}
	return model.PhotoMeta{}
}

func (fb *FileBrowser) RefreshItemMeta(displayIndex int, colors []model.ColorLabel, favorite bool) {
	fb.mu.Lock()
	allIdx := displayIndex
	if fb.indices != nil {
		if displayIndex >= 0 && displayIndex < len(fb.indices) {
			allIdx = fb.indices[displayIndex]
		} else {
			fb.mu.Unlock()
			return
		}
	}
	if allIdx >= 0 && allIdx < len(fb.allMeta) {
		fb.allMeta[allIdx].Colors = colors
		fb.allMeta[allIdx].Favorite = favorite
	}
	if displayIndex >= 0 && displayIndex < len(fb.meta) {
		fb.meta[displayIndex].Colors = colors
		fb.meta[displayIndex].Favorite = favorite
	}
	fb.mu.Unlock()
	fb.list.RefreshItem(displayIndex)
}

func (fb *FileBrowser) SelectIndex(index int) {
	if index >= 0 && index < len(fb.photos) {
		fb.list.Select(index)
	}
}

func (fb *FileBrowser) SetRoot(root string) {
	fb.dirTree.SetRoot(root)
}

func (fb *FileBrowser) OpenDirectory(path string) {
	fb.dirTree.OpenPath(path)
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

func (fb *FileBrowser) SetFilter(colors map[model.ColorLabel]bool, favorite bool) {
	copied := make(map[model.ColorLabel]bool, len(colors))
	for k, v := range colors {
		copied[k] = v
	}
	fb.mu.Lock()
	fb.filterColors = copied
	fb.filterFavorite = favorite
	fb.mu.Unlock()
	fb.applyFilter()
	fb.updateFilterButtonStates()
	fb.updateBulkBarVisibility()
	fb.list.Refresh()
}

func (fb *FileBrowser) FilteredPhotos() []model.Photo {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	result := make([]model.Photo, len(fb.photos))
	copy(result, fb.photos)
	return result
}

func (fb *FileBrowser) FilteredMeta() []model.PhotoMeta {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	result := make([]model.PhotoMeta, len(fb.meta))
	copy(result, fb.meta)
	return result
}

func (fb *FileBrowser) AllMeta() []model.PhotoMeta {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	result := make([]model.PhotoMeta, len(fb.allMeta))
	copy(result, fb.allMeta)
	return result
}

func (fb *FileBrowser) AllPhotos() []model.Photo {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	result := make([]model.Photo, len(fb.allPhotos))
	copy(result, fb.allPhotos)
	return result
}

func (fb *FileBrowser) RemovePhoto(imagePath string) {
	fb.mu.Lock()
	removeIdx := -1
	for i, p := range fb.allPhotos {
		if p.ImagePath == imagePath {
			removeIdx = i
			break
		}
	}
	if removeIdx < 0 {
		fb.mu.Unlock()
		return
	}
	fb.allPhotos = append(fb.allPhotos[:removeIdx], fb.allPhotos[removeIdx+1:]...)
	fb.allMeta = append(fb.allMeta[:removeIdx], fb.allMeta[removeIdx+1:]...)
	fb.mu.Unlock()
	fb.applyFilter()
	fb.updateBulkBarVisibility()
	fb.list.Refresh()
}

func (fb *FileBrowser) RemovePhotos(paths map[string]bool) {
	fb.mu.Lock()
	var newPhotos []model.Photo
	var newMeta []model.PhotoMeta
	for i, p := range fb.allPhotos {
		if !paths[p.ImagePath] {
			newPhotos = append(newPhotos, p)
			newMeta = append(newMeta, fb.allMeta[i])
		}
	}
	fb.allPhotos = newPhotos
	fb.allMeta = newMeta
	fb.mu.Unlock()
	fb.applyFilter()
	fb.updateBulkBarVisibility()
	fb.list.Refresh()
}

func (fb *FileBrowser) updateBulkBarVisibility() {
	fb.mu.Lock()
	active := HasActiveFilter(fb.filterColors, fb.filterFavorite)
	count := len(fb.photos)
	fb.mu.Unlock()

	if active && count > 0 {
		fb.bulkBar.Show()
	} else {
		fb.bulkBar.Hide()
	}
}

func (fb *FileBrowser) applyFilter() {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	if !HasActiveFilter(fb.filterColors, fb.filterFavorite) {
		fb.indices = nil
		fb.photos = fb.allPhotos
		fb.meta = fb.allMeta
		return
	}

	fb.indices = nil
	fb.photos = nil
	fb.meta = nil
	for i, m := range fb.allMeta {
		if fb.matchesFilter(m) || fb.allPhotos[i].ImagePath == fb.pinnedPath {
			fb.indices = append(fb.indices, i)
			fb.photos = append(fb.photos, fb.allPhotos[i])
			fb.meta = append(fb.meta, m)
		}
	}
}

func (fb *FileBrowser) matchesFilter(meta model.PhotoMeta) bool {
	if fb.filterFavorite && meta.Favorite {
		return true
	}
	colorSet := ColorSet(meta.Colors)
	for c, active := range fb.filterColors {
		if active && colorSet[c] {
			return true
		}
	}
	return false
}

func (fb *FileBrowser) displayIndex(allIdx int) int {
	if fb.indices == nil {
		return allIdx
	}
	for i, idx := range fb.indices {
		if idx == allIdx {
			return i
		}
	}
	return -1
}

func (fb *FileBrowser) updateFilterButtonStates() {
	setFilterBtnState(fb.filterFavBtn, fb.filterFavorite)
	setFilterBtnState(fb.filterRedBtn, fb.filterColors[model.ColorRed])
	setFilterBtnState(fb.filterGreenBtn, fb.filterColors[model.ColorGreen])
	setFilterBtnState(fb.filterBlueBtn, fb.filterColors[model.ColorBlue])
}

func HasActiveFilter(colors map[model.ColorLabel]bool, favorite bool) bool {
	if favorite {
		return true
	}
	for _, v := range colors {
		if v {
			return true
		}
	}
	return false
}

func setFilterBtnState(btn *widget.Button, active bool) {
	if active {
		btn.Importance = widget.HighImportance
	} else {
		btn.Importance = widget.MediumImportance
	}
	btn.Refresh()
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

	fb.filterFavBtn = widget.NewButtonWithIcon("", iconHeartOutline, func() {
		if fb.callbacks.OnFilterFavorite != nil {
			fb.callbacks.OnFilterFavorite()
		}
	})
	fb.filterFavBtn.Importance = widget.MediumImportance
	fb.filterRedBtn = widget.NewButtonWithIcon("", iconRedCircle, func() {
		if fb.callbacks.OnFilterRed != nil {
			fb.callbacks.OnFilterRed()
		}
	})
	fb.filterRedBtn.Importance = widget.MediumImportance
	fb.filterGreenBtn = widget.NewButtonWithIcon("", iconGreenCircle, func() {
		if fb.callbacks.OnFilterGreen != nil {
			fb.callbacks.OnFilterGreen()
		}
	})
	fb.filterGreenBtn.Importance = widget.MediumImportance
	fb.filterBlueBtn = widget.NewButtonWithIcon("", iconBlueCircle, func() {
		if fb.callbacks.OnFilterBlue != nil {
			fb.callbacks.OnFilterBlue()
		}
	})
	fb.filterBlueBtn.Importance = widget.MediumImportance
	filterBar := container.NewGridWithColumns(4, fb.filterFavBtn, fb.filterRedBtn, fb.filterGreenBtn, fb.filterBlueBtn)

	fb.deleteAllBtn = widget.NewButtonWithIcon("Delete All", theme.DeleteIcon(), func() {
		if fb.callbacks.OnDeleteAll != nil {
			fb.callbacks.OnDeleteAll()
		}
	})
	fb.deleteAllBtn.Importance = widget.DangerImportance
	fb.copyAllBtn = widget.NewButtonWithIcon("Copy All", theme.ContentCopyIcon(), func() {
		if fb.callbacks.OnCopyAll != nil {
			fb.callbacks.OnCopyAll()
		}
	})
	fb.bulkBar = container.NewGridWithColumns(2, fb.deleteAllBtn, fb.copyAllBtn)
	fb.bulkBar.Hide()

	topBars := container.NewVBox(sortBar, filterBar, fb.bulkBar)
	treeWithBtn := container.NewBorder(chooseBtn, nil, nil, nil, fb.dirTree.Widget())
	listWithSort := container.NewBorder(topBars, nil, nil, nil, fb.list)
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
