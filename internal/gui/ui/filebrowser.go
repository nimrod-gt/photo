package ui

import (
	"log"
	"maps"
	"path/filepath"
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"photo/internal/core/imaging"
	"photo/internal/core/library"
	"photo/internal/core/model"
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
	fb.refreshRows()

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
				fb.refreshRows()
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
	fb.updateFilterButtonStates()
	fb.refreshRows()
}

func (fb *FileBrowser) refreshRows() {
	fb.data.applyFilter()
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
	fb.refreshRows()
}

func (fb *FileBrowser) RemovePhotos(paths map[string]bool) {
	fb.data.removePhotos(paths)
	fb.refreshRows()
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
	return favorite || slices.Contains(slices.Collect(maps.Values(colors)), true)
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
	fb.buildList()
	fb.buildBulkBar()

	chooseBtn := widget.NewButton("Open Folder...", func() { call(fb.callbacks.OnChooseFolder) })
	helpBtn := iconButton("", theme.HelpIcon(), fb.callbacks.OnHelp)
	helpBtn.Importance = widget.LowImportance

	topBars := container.NewVBox(fb.buildSortBar(), fb.buildFilterBar(), fb.bulkBar)
	treeWithBtn := container.NewBorder(container.NewBorder(nil, nil, helpBtn, nil, chooseBtn), nil, nil, nil, fb.dirTree.Widget())
	listWithSort := container.NewBorder(topBars, nil, nil, nil, fb.list)
	split := container.NewVSplit(treeWithBtn, listWithSort)
	split.SetOffset(0.4)

	fb.container = container.NewStack(split)
}

func (fb *FileBrowser) buildList() {
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
}

func (fb *FileBrowser) buildSortBar() *fyne.Container {
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
	return container.NewGridWithColumns(2, fb.nameSortBtn, fb.timeSortBtn)
}

func (fb *FileBrowser) buildFilterBar() *fyne.Container {
	fb.filterFavBtn = filterButton(iconHeartOutline, fb.callbacks.OnFilterFavorite)
	fb.filterRedBtn = filterButton(iconRedCircle, fb.callbacks.OnFilterRed)
	fb.filterGreenBtn = filterButton(iconGreenCircle, fb.callbacks.OnFilterGreen)
	fb.filterBlueBtn = filterButton(iconBlueCircle, fb.callbacks.OnFilterBlue)
	return container.NewGridWithColumns(4, fb.filterFavBtn, fb.filterRedBtn, fb.filterGreenBtn, fb.filterBlueBtn)
}

func filterButton(icon fyne.Resource, fn func()) *widget.Button {
	btn := iconButton("", icon, fn)
	btn.Importance = widget.MediumImportance
	return btn
}

func (fb *FileBrowser) buildBulkBar() {
	fb.deleteAllBtn = iconButton("Delete All", theme.DeleteIcon(), fb.callbacks.OnDeleteAll)
	fb.deleteAllBtn.Importance = widget.DangerImportance
	fb.copyAllBtn = iconButton("Copy All", theme.ContentCopyIcon(), fb.callbacks.OnCopyAll)
	fb.unselectAllBtn = iconButton("Unselect All", theme.CancelIcon(), fb.callbacks.OnUnselectAll)
	fb.bulkBar = container.NewVBox(
		container.NewGridWithColumns(2, fb.deleteAllBtn, fb.copyAllBtn),
		fb.unselectAllBtn,
	)
	fb.bulkBar.Hide()
}
