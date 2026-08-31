package ui

import (
	"log"
	"path/filepath"

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
	OnFilterColor       func(color model.ColorLabel)
	OnFilterFavorite    func()
	OnFilteredChanged   func(photos []model.Photo)
	OnMetaLoaded        func(displayIndex int)
	OnDeleteAll         func()
	OnCopyAll           func()
	OnUnselectAll       func()
	OnHelp              func()
}

type FileBrowser struct {
	*photoList
	container   *fyne.Container
	list        *widget.List
	nameSortBtn *widget.Button
	timeSortBtn *widget.Button
	dirTree     *DirTree

	filterFavBtn    *widget.Button
	filterColorBtns map[model.ColorLabel]*widget.Button

	bulkBar        *fyne.Container
	deleteAllBtn   *widget.Button
	copyAllBtn     *widget.Button
	unselectAllBtn *widget.Button

	imageProvider *imaging.Provider
	colors        *library.ColorService
	callbacks     FileBrowserCallbacks
}

func NewFileBrowser(
	imageProvider *imaging.Provider,
	colors *library.ColorService,
	callbacks FileBrowserCallbacks,
) *FileBrowser {
	fb := &FileBrowser{
		photoList:     newPhotoList(),
		imageProvider: imageProvider,
		colors:        colors,
		callbacks:     callbacks,
	}
	fb.dirTree = NewDirTree(callbacks.OnDirectorySelected)
	fb.build()
	return fb
}

func (fb *FileBrowser) Container() *fyne.Container {
	return fb.container
}

func (fb *FileBrowser) SetPhotos(photos []model.Photo) {
	gen := fb.reset(photos)

	fb.loadInitialMeta(photos)
	fb.refreshRows()

	fb.imageProvider.LoadFolder(photos, func(index int, meta imaging.LoadedMeta) {
		displayIdx, ok := fb.setLoadedMeta(index, meta, gen)
		if ok && displayIdx >= 0 {
			fyne.Do(func() {
				fb.list.RefreshItem(displayIdx)
				if fb.callbacks.OnMetaLoaded != nil {
					fb.callbacks.OnMetaLoaded(displayIdx)
				}
			})
		}
	}, func() {
		if fb.favoriteRefilterNeeded(gen) {
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

	fb.initMeta(photos, colorMap)
}

func (fb *FileBrowser) RefreshItemMeta(displayIndex int, colors []model.ColorLabel, favorite bool) {
	fb.setItemMeta(displayIndex, colors, favorite)
	fb.list.RefreshItem(displayIndex)
}

// The colours of a photo change without its rating, which the folder scan may
// still be on its way to read.
func (fb *FileBrowser) RefreshItemColors(displayIndex int, colors []model.ColorLabel) {
	fb.setItemColors(displayIndex, colors)
	fb.list.RefreshItem(displayIndex)
}

func (fb *FileBrowser) SelectIndex(index int) {
	if index >= 0 && index < fb.count() {
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

func (fb *FileBrowser) ClearFilter() {
	fb.clearFilters()
	fb.RefreshFilter()
}

func (fb *FileBrowser) RefreshFilter() {
	fb.updateFilterButtonStates()
	fb.refreshRows()
}

func (fb *FileBrowser) refreshRows() {
	fb.applyFilter()
	fb.updateBulkBarVisibility()
	fb.list.Refresh()
}

func (fb *FileBrowser) RemovePhoto(imagePath string) {
	if !fb.removePhoto(imagePath) {
		return
	}
	fb.refreshRows()
}

func (fb *FileBrowser) RemovePhotos(paths map[string]bool) {
	fb.removePhotos(paths)
	fb.refreshRows()
}

func (fb *FileBrowser) updateBulkBarVisibility() {
	active, colorActive, count := fb.bulkState()
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
	filterColors, filterFavorite := fb.filterState()
	setFilterBtnState(fb.filterFavBtn, filterFavorite)
	for label, btn := range fb.filterColorBtns {
		setFilterBtnState(btn, filterColors[label])
	}
}

func HasActiveFilter(colors map[model.ColorLabel]bool, favorite bool) bool {
	if favorite {
		return true
	}
	for _, active := range colors {
		if active {
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
		fb.count,
		fb.createItem,
		fb.updateItem,
	)
	fb.list.OnSelected = func(id widget.ListItemID) {
		photo, ok := fb.photoAt(id)
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
	buttons := make([]fyne.CanvasObject, 0, len(ColorOrder)+1)
	buttons = append(buttons, fb.filterFavBtn)
	fb.filterColorBtns = make(map[model.ColorLabel]*widget.Button, len(ColorOrder))
	for _, label := range ColorOrder {
		btn := filterButton(colorIcons[label], func() { callWith(fb.callbacks.OnFilterColor, label) })
		fb.filterColorBtns[label] = btn
		buttons = append(buttons, btn)
	}
	return container.NewGridWithColumns(len(buttons), buttons...)
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
