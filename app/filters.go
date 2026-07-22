package app

import (
	"slices"
	"time"

	"photo/model"
	"photo/service"
)

func (a *Application) handleSortBy(order service.SortOrder) {
	if a.sortOrder == order {
		a.sortDescending = !a.sortDescending
	} else {
		a.sortOrder = order
		a.sortDescending = false
	}
	a.resortPhotos()
}

func (a *Application) handleSortToggle() {
	if a.sortDescending {
		if a.sortOrder == service.SortByName {
			a.sortOrder = service.SortByTime
		} else {
			a.sortOrder = service.SortByName
		}
		a.sortDescending = false
	} else {
		a.sortDescending = true
	}
	a.resortPhotos()
}

func (a *Application) resortPhotos() {
	photos := a.fileBrowser.AllPhotos()
	if a.sortOrder == service.SortByTime {
		allMeta := a.fileBrowser.AllMeta()
		dates := make(map[string]time.Time, len(photos))
		for i, p := range photos {
			if i < len(allMeta) {
				dates[p.ImagePath] = allMeta[i].Date
			}
		}
		a.scanner.SortPhotosByDates(photos, dates)
	} else {
		a.scanner.SortPhotos(photos, a.sortOrder)
	}
	if a.sortDescending {
		slices.Reverse(photos)
	}
	a.fileBrowser.SetPhotos(photos)
	a.fileBrowser.SetSortState(a.sortOrder, a.sortDescending)

	filtered := a.fileBrowser.FilteredPhotos()
	a.navigator.SetPhotos(filtered)
	if a.gridMode {
		a.enterGridMode()
		return
	}
	a.showCurrentOrFirst()
}

func (a *Application) unpinIfMovedAway(currentPhoto model.Photo) {
	pinnedPath := a.fileBrowser.PinnedPath()
	if len(pinnedPath) == 0 || !a.fileBrowser.HasFilter() {
		return
	}
	if currentPhoto.ImagePath == pinnedPath {
		return
	}
	a.fileBrowser.ClearPinnedPath()
	a.reapplyFilter()
}

func (a *Application) handleFilterColor(color model.ColorLabel) {
	a.fileBrowser.ToggleColorFilter(color)
	a.fileBrowser.ClearPinnedPath()
	a.reapplyFilter()
}

func (a *Application) handleFilterFavorite() {
	a.fileBrowser.ToggleFavoriteFilter()
	a.fileBrowser.ClearPinnedPath()
	a.reapplyFilter()
}

func (a *Application) reapplyFilter() {
	a.fileBrowser.RefreshFilter()
	a.syncNavigatorToFiltered()
	if a.gridMode {
		a.enterGridMode()
	}
}

func (a *Application) handleFilteredChanged(photos []model.Photo) {
	a.navigator.SetPhotos(photos)
	if a.gridMode {
		a.enterGridMode()
		return
	}
	a.showCurrentOrFirst()
}

func (a *Application) syncNavigatorToFiltered() {
	var currentPath string
	if cur, ok := a.navigator.Current(); ok {
		currentPath = cur.ImagePath
	}

	filtered := a.fileBrowser.FilteredPhotos()
	a.navigator.SetPhotos(filtered)

	if len(currentPath) > 0 {
		idx := a.navigator.FindIndex(currentPath)
		if idx >= 0 {
			if p, _, ok := a.navigator.GoTo(idx); ok {
				a.showPhoto(p)
				a.fileBrowser.SelectIndex(idx)
				return
			}
		}
	}

	a.showCurrentOrFirst()
}
