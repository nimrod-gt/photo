package ui

import (
	"image"
	"sync"

	"photo/model"
)

type photoList struct {
	mu             sync.Mutex
	allPhotos      []model.Photo
	allMeta        []model.PhotoMeta
	photos         []model.Photo
	meta           []model.PhotoMeta
	indices        []int
	displayByAll   map[int]int
	filterColors   map[model.ColorLabel]bool
	filterFavorite bool
	pinnedPath     string
	generation     uint64
}

func newPhotoList() *photoList {
	return &photoList{
		filterColors: make(map[model.ColorLabel]bool),
	}
}

func (pl *photoList) reset(photos []model.Photo) uint64 {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	pl.allPhotos = photos
	pl.allMeta = make([]model.PhotoMeta, len(photos))
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
	}
}

func (pl *photoList) setLoadedMeta(index int, thumbnail image.Image, favorite bool, gen uint64) (int, bool) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	if pl.generation != gen {
		return -1, false
	}
	if index < len(pl.allMeta) {
		pl.allMeta[index].Thumbnail = thumbnail
		pl.allMeta[index].Favorite = favorite
	}
	return pl.displayIndex(index), true
}

func (pl *photoList) favoriteRefilterNeeded(gen uint64) bool {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	return pl.generation == gen && pl.filterFavorite
}

func (pl *photoList) setPinnedPath(path string) {
	pl.mu.Lock()
	pl.pinnedPath = path
	pl.mu.Unlock()
}

func (pl *photoList) getPinnedPath() string {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	return pl.pinnedPath
}

func (pl *photoList) hasFilter() bool {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	return HasActiveFilter(pl.filterColors, pl.filterFavorite)
}

func (pl *photoList) toggleColorFilter(color model.ColorLabel) {
	pl.mu.Lock()
	pl.filterColors[color] = !pl.filterColors[color]
	pl.mu.Unlock()
}

func (pl *photoList) toggleFavoriteFilter() {
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
	for k, v := range pl.filterColors {
		colors[k] = v
	}
	return colors, pl.filterFavorite
}

func (pl *photoList) bulkState() (active bool, count int) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	return HasActiveFilter(pl.filterColors, pl.filterFavorite), len(pl.photos)
}

func (pl *photoList) applyFilter() {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	if !HasActiveFilter(pl.filterColors, pl.filterFavorite) {
		pl.indices = nil
		pl.displayByAll = nil
		pl.photos = pl.allPhotos
		pl.meta = pl.allMeta
		return
	}

	pl.indices = nil
	pl.displayByAll = make(map[int]int)
	pl.photos = nil
	pl.meta = nil
	for i, m := range pl.allMeta {
		if pl.matchesFilter(m) || pl.allPhotos[i].ImagePath == pl.pinnedPath {
			pl.displayByAll[i] = len(pl.indices)
			pl.indices = append(pl.indices, i)
			pl.photos = append(pl.photos, pl.allPhotos[i])
			pl.meta = append(pl.meta, m)
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

func (pl *photoList) displayIndex(allIdx int) int {
	if pl.indices == nil {
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
	return len(pl.photos)
}

func (pl *photoList) photoAt(displayIndex int) (model.Photo, bool) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	if displayIndex >= 0 && displayIndex < len(pl.photos) {
		return pl.photos[displayIndex], true
	}
	return model.Photo{}, false
}

func (pl *photoList) itemAt(displayIndex int) (model.Photo, model.PhotoMeta, bool) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	if displayIndex >= 0 && displayIndex < len(pl.photos) {
		return pl.photos[displayIndex], pl.meta[displayIndex], true
	}
	return model.Photo{}, model.PhotoMeta{}, false
}

func (pl *photoList) metaAt(displayIndex int) model.PhotoMeta {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	if displayIndex >= 0 && displayIndex < len(pl.meta) {
		return pl.meta[displayIndex]
	}
	return model.PhotoMeta{}
}

func (pl *photoList) setItemMeta(displayIndex int, colors []model.ColorLabel, favorite bool) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	allIdx := displayIndex
	if pl.indices != nil {
		if displayIndex >= 0 && displayIndex < len(pl.indices) {
			allIdx = pl.indices[displayIndex]
		} else {
			return
		}
	}
	if allIdx >= 0 && allIdx < len(pl.allMeta) {
		pl.allMeta[allIdx].Colors = colors
		pl.allMeta[allIdx].Favorite = favorite
	}
	if displayIndex >= 0 && displayIndex < len(pl.meta) {
		pl.meta[displayIndex].Colors = colors
		pl.meta[displayIndex].Favorite = favorite
	}
}

func (pl *photoList) filteredPhotos() []model.Photo {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	result := make([]model.Photo, len(pl.photos))
	copy(result, pl.photos)
	return result
}

func (pl *photoList) filteredMeta() []model.PhotoMeta {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	result := make([]model.PhotoMeta, len(pl.meta))
	copy(result, pl.meta)
	return result
}

func (pl *photoList) allMetaCopy() []model.PhotoMeta {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	result := make([]model.PhotoMeta, len(pl.allMeta))
	copy(result, pl.allMeta)
	return result
}

func (pl *photoList) allPhotosCopy() []model.Photo {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	result := make([]model.Photo, len(pl.allPhotos))
	copy(result, pl.allPhotos)
	return result
}

func (pl *photoList) removePhoto(imagePath string) bool {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	removeIdx := -1
	for i, p := range pl.allPhotos {
		if p.ImagePath == imagePath {
			removeIdx = i
			break
		}
	}
	if removeIdx < 0 {
		return false
	}
	pl.allPhotos = append(pl.allPhotos[:removeIdx], pl.allPhotos[removeIdx+1:]...)
	pl.allMeta = append(pl.allMeta[:removeIdx], pl.allMeta[removeIdx+1:]...)
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
}
