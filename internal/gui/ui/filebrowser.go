package ui

import (
	"image"
	"image/color"
	"log"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"photo/internal/core/imaging"
	"photo/internal/core/library"
	"photo/internal/core/model"
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
	OnSortBy            func(order library.SortOrder)
	OnFilterRed         func()
	OnFilterGreen       func()
	OnFilterBlue        func()
	OnFilterFavorite    func()
	OnFilteredChanged   func(photos []model.Photo)
	OnMetaLoaded        func(displayIndex int)
	OnDeleteAll         func()
	OnCopyAll           func()
	OnUnselectAll       func()
	OnHelp              func()
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

	bulkBar        *fyne.Container
	deleteAllBtn   *widget.Button
	copyAllBtn     *widget.Button
	unselectAllBtn *widget.Button

	data          *photoList
	items         map[fyne.CanvasObject]*browserItem
	imageProvider *imaging.Provider
	colors        *library.ColorService
	callbacks     FileBrowserCallbacks
}

type browserItem struct {
	thumb    *canvas.Image
	name     *widget.Label
	star     *canvas.Text
	dots     *fyne.Container
	dateText *canvas.Text
}

func NewFileBrowser(
	scanner *library.Scanner,
	imageProvider *imaging.Provider,
	colors *library.ColorService,
	callbacks FileBrowserCallbacks,
) *FileBrowser {
	fb := &FileBrowser{
		imageProvider: imageProvider,
		colors:        colors,
		callbacks:     callbacks,
		data:          newPhotoList(),
		items:         make(map[fyne.CanvasObject]*browserItem),
	}
	fb.dirTree = NewDirTree(scanner, callbacks.OnDirectorySelected)
	fb.build()
	return fb
}

func (fb *FileBrowser) Container() *fyne.Container {
	return fb.container
}

func (fb *FileBrowser) SetPinnedPath(path string) {
	fb.data.setPinnedPath(path)
}

func (fb *FileBrowser) ClearPinnedPath() {
	fb.data.setPinnedPath("")
}

func (fb *FileBrowser) PinnedPath() string {
	return fb.data.getPinnedPath()
}

func (fb *FileBrowser) SetPhotos(photos []model.Photo) {
	gen := fb.data.reset(photos)

	fb.loadInitialMeta(photos)
	fb.data.applyFilter()
	fb.updateBulkBarVisibility()
	fb.list.Refresh()

	fb.imageProvider.LoadFolder(photos, func(index int, meta imaging.LoadedMeta) {
		displayIdx, ok := fb.data.setLoadedMeta(index, meta, gen)
		if ok && displayIdx >= 0 {
			fyne.Do(func() {
				fb.list.RefreshItem(displayIdx)
				if fb.callbacks.OnMetaLoaded != nil {
					fb.callbacks.OnMetaLoaded(displayIdx)
				}
			})
		}
	}, func() {
		if fb.data.favoriteRefilterNeeded(gen) {
			fyne.Do(func() {
				fb.data.applyFilter()
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

	fb.data.initMeta(photos, colorMap)
}

func (fb *FileBrowser) GetMeta(displayIndex int) model.PhotoMeta {
	return fb.data.metaAt(displayIndex)
}

func (fb *FileBrowser) RefreshItemMeta(displayIndex int, colors []model.ColorLabel, favorite bool) {
	fb.data.setItemMeta(displayIndex, colors, favorite)
	fb.list.RefreshItem(displayIndex)
}

// The colours of a photo change without its rating, which the folder scan may
// still be on its way to read.
func (fb *FileBrowser) RefreshItemColors(displayIndex int, colors []model.ColorLabel) {
	fb.data.setItemColors(displayIndex, colors)
	fb.list.RefreshItem(displayIndex)
}

func (fb *FileBrowser) SelectIndex(index int) {
	if index >= 0 && index < fb.data.count() {
		fb.list.Select(index)
	}
}

func (fb *FileBrowser) SetRoot(root string) {
	fb.dirTree.SetRoot(root)
}

func (fb *FileBrowser) OpenDirectory(path string) {
	fb.dirTree.OpenPath(path)
}

func (fb *FileBrowser) SetSortState(order library.SortOrder, descending bool) {
	arrow := " ↑"
	if descending {
		arrow = " ↓"
	}
	if order == library.SortByName {
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

func (fb *FileBrowser) HasFilter() bool {
	return fb.data.hasFilter()
}

func (fb *FileBrowser) ToggleColorFilter(color model.ColorLabel) {
	fb.data.toggleColorFilter(color)
}

func (fb *FileBrowser) ToggleFavoriteFilter() {
	fb.data.toggleFavoriteFilter()
}

func (fb *FileBrowser) ClearFilter() {
	fb.data.clearFilters()
	fb.RefreshFilter()
}

func (fb *FileBrowser) RefreshFilter() {
	fb.data.applyFilter()
	fb.updateFilterButtonStates()
	fb.updateBulkBarVisibility()
	fb.list.Refresh()
}

func (fb *FileBrowser) ActiveFilterColors() []model.ColorLabel {
	return fb.data.activeFilterColors()
}

func (fb *FileBrowser) RemoveColorLabels(paths map[string]bool, colors []model.ColorLabel) {
	fb.data.removeColorsFromPaths(paths, colors)
}

func (fb *FileBrowser) FilteredPhotos() []model.Photo {
	return fb.data.filteredPhotos()
}

func (fb *FileBrowser) FilteredMeta() []model.PhotoMeta {
	return fb.data.filteredMeta()
}

func (fb *FileBrowser) AllMeta() []model.PhotoMeta {
	return fb.data.allMetaCopy()
}

func (fb *FileBrowser) AllPhotos() []model.Photo {
	return fb.data.allPhotosCopy()
}

func (fb *FileBrowser) RemovePhoto(imagePath string) {
	if !fb.data.removePhoto(imagePath) {
		return
	}
	fb.data.applyFilter()
	fb.updateBulkBarVisibility()
	fb.list.Refresh()
}

func (fb *FileBrowser) RemovePhotos(paths map[string]bool) {
	fb.data.removePhotos(paths)
	fb.data.applyFilter()
	fb.updateBulkBarVisibility()
	fb.list.Refresh()
}

func (fb *FileBrowser) updateBulkBarVisibility() {
	active, colorActive, count := fb.data.bulkState()
	if active && count > 0 {
		fb.bulkBar.Show()
	} else {
		fb.bulkBar.Hide()
	}
	if colorActive {
		fb.unselectAllBtn.Show()
	} else {
		fb.unselectAllBtn.Hide()
	}
}

func (fb *FileBrowser) updateFilterButtonStates() {
	filterColors, filterFavorite := fb.data.filterState()
	setFilterBtnState(fb.filterFavBtn, filterFavorite)
	setFilterBtnState(fb.filterRedBtn, filterColors[model.ColorRed])
	setFilterBtnState(fb.filterGreenBtn, filterColors[model.ColorGreen])
	setFilterBtnState(fb.filterBlueBtn, filterColors[model.ColorBlue])
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
		fb.data.count,
		fb.createItem,
		fb.updateItem,
	)

	fb.list.OnSelected = func(id widget.ListItemID) {
		photo, ok := fb.data.photoAt(id)
		if ok && fb.callbacks.OnPhotoSelected != nil {
			fb.callbacks.OnPhotoSelected(photo)
		}
	}

	chooseBtn := widget.NewButton("Open Folder...", func() {
		if fb.callbacks.OnChooseFolder != nil {
			fb.callbacks.OnChooseFolder()
		}
	})

	helpBtn := widget.NewButtonWithIcon("", theme.HelpIcon(), func() {
		if fb.callbacks.OnHelp != nil {
			fb.callbacks.OnHelp()
		}
	})
	helpBtn.Importance = widget.LowImportance

	fb.nameSortBtn = widget.NewButton("Name ↑", func() {
		if fb.callbacks.OnSortBy != nil {
			fb.callbacks.OnSortBy(library.SortByName)
		}
	})
	fb.nameSortBtn.Importance = widget.HighImportance
	fb.timeSortBtn = widget.NewButton("Time", func() {
		if fb.callbacks.OnSortBy != nil {
			fb.callbacks.OnSortBy(library.SortByTime)
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
	fb.unselectAllBtn = widget.NewButtonWithIcon("Unselect All", theme.CancelIcon(), func() {
		if fb.callbacks.OnUnselectAll != nil {
			fb.callbacks.OnUnselectAll()
		}
	})
	fb.bulkBar = container.NewVBox(
		container.NewGridWithColumns(2, fb.deleteAllBtn, fb.copyAllBtn),
		fb.unselectAllBtn,
	)
	fb.bulkBar.Hide()

	topBars := container.NewVBox(sortBar, filterBar, fb.bulkBar)
	treeWithBtn := container.NewBorder(container.NewBorder(nil, nil, helpBtn, nil, chooseBtn), nil, nil, nil, fb.dirTree.Widget())
	listWithSort := container.NewBorder(topBars, nil, nil, nil, fb.list)
	split := container.NewVSplit(treeWithBtn, listWithSort)
	split.SetOffset(0.4)

	fb.container = container.NewStack(split)
}

const (
	favoriteMark = "★"
	colorMark    = "●"
)

var favoriteColor = color.NRGBA{R: 255, G: 215, B: 0, A: 255}

func (fb *FileBrowser) createItem() fyne.CanvasObject {
	thumb := canvas.NewImageFromImage(nil)
	thumb.SetMinSize(fyne.NewSize(thumbnailWidth, thumbnailHeight))
	thumb.FillMode = canvas.ImageFillContain

	nameLabel := widget.NewLabel("placeholder")
	nameLabel.Truncation = fyne.TextTruncateEllipsis

	star := canvas.NewText(favoriteMark, favoriteColor)
	star.TextSize = 14
	star.Hide()

	dot1 := canvas.NewText(colorMark, color.Transparent)
	dot1.TextSize = 12
	dot1.Hide()
	dot2 := canvas.NewText(colorMark, color.Transparent)
	dot2.TextSize = 12
	dot2.Hide()
	dot3 := canvas.NewText(colorMark, color.Transparent)
	dot3.TextSize = 12
	dot3.Hide()

	dotsContainer := container.NewHBox(dot1, dot2, dot3)

	topRow := container.NewBorder(nil, nil, nil, star, nameLabel)

	dateText := canvas.NewText("", color.NRGBA{R: 160, G: 160, B: 160, A: 255})
	dateText.TextSize = dateFontSize
	dateContent := container.New(layout.NewCustomPaddedLayout(0, 0, theme.InnerPadding(), 0), dateText)
	dateRow := container.NewBorder(nil, nil, nil, dotsContainer, dateContent)

	textBox := container.NewVBox(topRow, dateRow)

	root := container.NewBorder(nil, nil, thumb, nil,
		container.New(layout.NewStackLayout(), textBox),
	)
	fb.items[root] = &browserItem{
		thumb:    thumb,
		name:     nameLabel,
		star:     star,
		dots:     dotsContainer,
		dateText: dateText,
	}
	return root
}

func (fb *FileBrowser) updateItem(id widget.ListItemID, obj fyne.CanvasObject) {
	photo, meta, ok := fb.data.itemAt(id)
	if !ok {
		return
	}

	item, ok := fb.items[obj]
	if !ok {
		return
	}

	thumbnail := meta.Thumbnail
	if thumbnail == nil {
		thumbnail = fb.imageProvider.Thumbnail(photo.ImagePath)
	}
	updateThumbnail(item.thumb, thumbnail)
	item.name.SetText(photo.Name)
	updateStar(item.star, meta.Favorite)
	updateColorDots(item.dots, meta.Colors)
	updateDateText(item.dateText, meta.Date)
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
