package ui

import (
	"maps"
	"slices"
	"sync"

	"photo/internal/core/imaging"
	"photo/internal/core/model"
)

type photoList struct {
	mu        sync.Mutex
	allPhotos []model.Photo
	allMeta   []model.PhotoMeta
	// The folder as the scan reading it was handed it. The list owns a copy of
	// its own because the scan runs on its goroutine without this lock and would
	// be reading the array a delete is rewriting; it keeps the scan's own order
	// because the index a scan reports is an index into what it was given, which
	// a delete no longer lines up with.
	scanPhotos []model.Photo
	// Built the first time a scan reports on a folder a delete has shortened,
	// and dropped by the next one: walking the folder per photo read would cost
	// the square of it.
	allByPath map[string]int
	// A filter that matches nothing leaves indices empty, which is not the same
	// as the whole folder: displayByAll is nil only while no filter is on, so
	// it says which of the two it is rather than leaving an empty slice to
	// speak for both.
	indices      []int
	displayByAll map[int]int
	// The rating of a photo the user toggled is already on disk and in allMeta,
	// while the folder scan is still carrying the one it read before the toggle
	// and would put it back.
	toggledFavorites map[string]bool
	filterColors     map[model.ColorLabel]bool
	filterFavorite   bool
	pinnedPath       string
	generation       uint64
}

func newPhotoList() *photoList {
	return &photoList{
		filterColors: make(map[model.ColorLabel]bool),
	}
}

func (pl *photoList) reset(photos []model.Photo) uint64 {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	pl.allPhotos = slices.Clone(photos)
	pl.scanPhotos = photos
	pl.allByPath = nil
	pl.allMeta = make([]model.PhotoMeta, len(photos))
	pl.toggledFavorites = nil
	pl.pinnedPath = ""
	pl.generation++
	return pl.generation
}

func (pl *photoList) initMeta(photos []model.Photo, colorMap model.ColorMap) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	for i, photo := range photos {
		if colorMap != nil {
			pl.allMeta[i].Colors = colorMap[photo.Name]
		}
		pl.allMeta[i].Date = photo.ModTime
		// Until the scan has read the file, any JPEG may take a rating; the
		// scan refines this to the packet it actually finds.
		pl.allMeta[i].Ratable = photo.IsJPEG()
	}
}

func (pl *photoList) setLoadedMeta(index int, meta imaging.LoadedMeta, gen uint64) (int, bool) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	if pl.generation != gen {
		return -1, false
	}
	allIdx := pl.scanTarget(index)
	if allIdx < 0 {
		return -1, true
	}
	pl.allMeta[allIdx].Thumbnail = meta.Thumbnail
	pl.allMeta[allIdx].Ratable = meta.Ratable
	if !pl.toggledFavorites[pl.allPhotos[allIdx].ImagePath] {
		pl.allMeta[allIdx].Favorite = meta.Favorite
	}
	return pl.displayIndex(allIdx), true
}

// The place in the list of the photo a scan reports by its own index. Nothing
// is ever added to the list, so a length still matching the scan's is a folder
// nothing was deleted from and the index stands as it is; a photo deleted since
// has no place left and is answered with -1.
func (pl *photoList) scanTarget(index int) int {
	if index < 0 || index >= len(pl.scanPhotos) {
		return -1
	}
	if len(pl.allPhotos) == len(pl.scanPhotos) {
		return index
	}
	if pl.allByPath == nil {
		pl.allByPath = make(map[string]int, len(pl.allPhotos))
		for i, photo := range pl.allPhotos {
			pl.allByPath[photo.ImagePath] = i
		}
	}
	allIdx, ok := pl.allByPath[pl.scanPhotos[index].ImagePath]
	if !ok {
		return -1
	}
	return allIdx
}

func (pl *photoList) favoriteRefilterNeeded(gen uint64) bool {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	return pl.generation == gen && pl.filterFavorite
}

func (pl *photoList) SetPinnedPath(path string) {
	pl.mu.Lock()
	pl.pinnedPath = path
	pl.mu.Unlock()
}

func (pl *photoList) ClearPinnedPath() {
	pl.SetPinnedPath("")
}

func (pl *photoList) PinnedPath() string {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	return pl.pinnedPath
}

func (pl *photoList) HasFilter() bool {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	return HasActiveFilter(pl.filterColors, pl.filterFavorite)
}

func (pl *photoList) ToggleColorFilter(color model.ColorLabel) {
	pl.mu.Lock()
	pl.filterColors[color] = !pl.filterColors[color]
	pl.mu.Unlock()
}

func (pl *photoList) ToggleFavoriteFilter() {
	pl.mu.Lock()
	pl.filterFavorite = !pl.filterFavorite
	pl.mu.Unlock()
}

func (pl *photoList) clearFilters() {
	pl.mu.Lock()
	pl.filterFavorite = false
	for k := range pl.filterColors {
		pl.filterColors[k] = false
	}
	pl.mu.Unlock()
}

func (pl *photoList) filterState() (map[model.ColorLabel]bool, bool) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	colors := make(map[model.ColorLabel]bool, len(pl.filterColors))
	maps.Copy(colors, pl.filterColors)
	return colors, pl.filterFavorite
}

func (pl *photoList) bulkState() (active bool, colorActive bool, count int) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	return HasActiveFilter(pl.filterColors, pl.filterFavorite),
		HasActiveFilter(pl.filterColors, false),
		pl.displayed()
}

func (pl *photoList) ActiveFilterColors() []model.ColorLabel {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	var colors []model.ColorLabel
	for _, c := range ColorOrder {
		if pl.filterColors[c] {
			colors = append(colors, c)
		}
	}
	return colors
}

func (pl *photoList) RemoveColorLabels(paths map[string]bool, colors []model.ColorLabel) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	for i, p := range pl.allPhotos {
		if !paths[p.ImagePath] {
			continue
		}
		remaining := slices.DeleteFunc(pl.allMeta[i].Colors, func(c model.ColorLabel) bool {
			return slices.Contains(colors, c)
		})
		if len(remaining) == 0 {
			remaining = nil
		}
		pl.allMeta[i].Colors = remaining
	}
}

func (pl *photoList) applyFilter() {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	if !HasActiveFilter(pl.filterColors, pl.filterFavorite) {
		pl.indices = nil
		pl.displayByAll = nil
		return
	}

	pl.indices = nil
	pl.displayByAll = make(map[int]int)
	for i, m := range pl.allMeta {
		if pl.matchesFilter(m) || pl.allPhotos[i].ImagePath == pl.pinnedPath {
			pl.displayByAll[i] = len(pl.indices)
			pl.indices = append(pl.indices, i)
		}
	}
}

func (pl *photoList) matchesFilter(meta model.PhotoMeta) bool {
	if pl.filterFavorite && meta.Favorite {
		return true
	}
	colorSet := ColorSet(meta.Colors)
	for c, active := range pl.filterColors {
		if active && colorSet[c] {
			return true
		}
	}
	return false
}

// The rows on screen are the photos the filter kept, addressed through indices
// rather than copied: a copy of the meta would go stale as soon as the folder
// scan filled in a thumbnail or a rating behind the filter.
func (pl *photoList) filtered() bool {
	return pl.displayByAll != nil
}

func (pl *photoList) displayed() int {
	if !pl.filtered() {
		return len(pl.allPhotos)
	}
	return len(pl.indices)
}

func (pl *photoList) allIndex(displayIndex int) int {
	if displayIndex < 0 || displayIndex >= pl.displayed() {
		return -1
	}
	if !pl.filtered() {
		return displayIndex
	}
	return pl.indices[displayIndex]
}

func (pl *photoList) displayIndex(allIdx int) int {
	if !pl.filtered() {
		return allIdx
	}
	if i, ok := pl.displayByAll[allIdx]; ok {
		return i
	}
	return -1
}

func (pl *photoList) count() int {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	return pl.displayed()
}

func (pl *photoList) photoAt(displayIndex int) (model.Photo, bool) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	allIdx := pl.allIndex(displayIndex)
	if allIdx < 0 {
		return model.Photo{}, false
	}
	return pl.allPhotos[allIdx], true
}

func (pl *photoList) itemAt(displayIndex int) (model.Photo, model.PhotoMeta, bool) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	allIdx := pl.allIndex(displayIndex)
	if allIdx < 0 {
		return model.Photo{}, model.PhotoMeta{}, false
	}
	return pl.allPhotos[allIdx], pl.allMeta[allIdx], true
}

func (pl *photoList) GetMeta(displayIndex int) model.PhotoMeta {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	allIdx := pl.allIndex(displayIndex)
	if allIdx < 0 {
		return model.PhotoMeta{}
	}
	return pl.allMeta[allIdx]
}

func (pl *photoList) setItemColors(displayIndex int, colors []model.ColorLabel) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	if allIdx := pl.allIndex(displayIndex); allIdx >= 0 {
		pl.allMeta[allIdx].Colors = colors
	}
}

// The rating written here is the one the app just put into the file, so the
// scan is held off this photo from now on.
func (pl *photoList) setItemMeta(displayIndex int, colors []model.ColorLabel, favorite bool) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	allIdx := pl.allIndex(displayIndex)
	if allIdx < 0 {
		return
	}
	pl.allMeta[allIdx].Colors = colors
	pl.allMeta[allIdx].Favorite = favorite
	pl.allMeta[allIdx].Ratable = true
	if pl.toggledFavorites == nil {
		pl.toggledFavorites = make(map[string]bool)
	}
	pl.toggledFavorites[pl.allPhotos[allIdx].ImagePath] = true
}

func (pl *photoList) FilteredPhotos() []model.Photo {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	result := make([]model.Photo, 0, pl.displayed())
	for i := range pl.displayed() {
		result = append(result, pl.allPhotos[pl.allIndex(i)])
	}
	return result
}

func (pl *photoList) FilteredMeta() []model.PhotoMeta {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	result := make([]model.PhotoMeta, 0, pl.displayed())
	for i := range pl.displayed() {
		result = append(result, pl.allMeta[pl.allIndex(i)])
	}
	return result
}

func (pl *photoList) AllMeta() []model.PhotoMeta {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	return slices.Clone(pl.allMeta)
}

func (pl *photoList) AllPhotos() []model.Photo {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	return slices.Clone(pl.allPhotos)
}

func (pl *photoList) removePhoto(imagePath string) bool {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	removeIdx := slices.IndexFunc(pl.allPhotos, func(p model.Photo) bool { return p.ImagePath == imagePath })
	if removeIdx < 0 {
		return false
	}
	pl.allPhotos = slices.Delete(pl.allPhotos, removeIdx, removeIdx+1)
	pl.allMeta = slices.Delete(pl.allMeta, removeIdx, removeIdx+1)
	pl.allByPath = nil
	return true
}

func (pl *photoList) removePhotos(paths map[string]bool) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	var newPhotos []model.Photo
	var newMeta []model.PhotoMeta
	for i, p := range pl.allPhotos {
		if !paths[p.ImagePath] {
			newPhotos = append(newPhotos, p)
			newMeta = append(newMeta, pl.allMeta[i])
		}
	}
	pl.allPhotos = newPhotos
	pl.allMeta = newMeta
	pl.allByPath = nil
}
