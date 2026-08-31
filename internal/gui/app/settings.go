package app

import (
	"photo/internal/core/library"
)

const (
	sortOrderKey      = "sortOrder"
	sortDescendingKey = "sortDescending"
	showTagsKey       = "showTags"
	autoSaveXMPKey    = "autoSaveXMP"
	autoSaveJPEGKey   = "autoSaveJPEG"
	showSaveButtonKey = "showSaveButton"
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

func (a *Application) restoreTagVisibility() {
	a.showTags = a.fyneApp.Preferences().BoolWithFallback(showTagsKey, true)
	a.tagBar.SetEnabled(a.showTags)
}

func (a *Application) saveTagVisibility() {
	a.fyneApp.Preferences().SetBool(showTagsKey, a.showTags)
}

func (a *Application) restoreSaveSettings() {
	prefs := a.fyneApp.Preferences()
	a.autoSaveXMP = prefs.BoolWithFallback(autoSaveXMPKey, true)
	a.autoSaveJPEG = prefs.BoolWithFallback(autoSaveJPEGKey, false)
	a.showSaveButton = prefs.BoolWithFallback(showSaveButtonKey, true)
}

func (a *Application) boolSetting(field *bool, key string) func(bool) {
	return func(on bool) {
		*field = on
		a.fyneApp.Preferences().SetBool(key, on)
	}
}

// Both files write themselves, so there is nothing left for the button to do
// and it goes rather than standing there as a no-op.
func (a *Application) saveButtonVisible() bool {
	return a.showSaveButton && (!a.autoSaveXMP || !a.autoSaveJPEG)
}

func (a *Application) saveSortOrder() {
	prefs := a.fyneApp.Preferences()
	prefs.SetInt(sortOrderKey, int(a.sortOrder))
	prefs.SetBool(sortDescendingKey, a.sortDescending)
}
