package app

import (
	"photo/internal/core/library"
)

const (
	sortOrderKey      = "sortOrder"
	sortDescendingKey = "sortDescending"
)

func normalizeSortOrder(value int) library.SortOrder {
	switch library.SortOrder(value) {
	case library.SortByName, library.SortByTime:
		return library.SortOrder(value)
	default:
		return library.SortByName
	}
}

func (a *Application) restoreSortOrder() {
	prefs := a.fyneApp.Preferences()
	a.sortOrder = normalizeSortOrder(prefs.IntWithFallback(sortOrderKey, int(library.SortByName)))
	a.sortDescending = prefs.BoolWithFallback(sortDescendingKey, false)
	a.fileBrowser.SetSortState(a.sortOrder, a.sortDescending)
}

func (a *Application) saveSortOrder() {
	prefs := a.fyneApp.Preferences()
	prefs.SetInt(sortOrderKey, int(a.sortOrder))
	prefs.SetBool(sortDescendingKey, a.sortDescending)
}
