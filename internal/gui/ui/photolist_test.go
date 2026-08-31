package ui

import (
	"image"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"photo/internal/core/imaging"
	"photo/internal/core/model"
)

func testPhotos(names ...string) []model.Photo {
	photos := make([]model.Photo, 0, len(names))
	for _, name := range names {
		photos = append(photos, model.Photo{ImagePath: filepath.Join("/photos", name), Name: name})
	}
	return photos
}

// The folder scan runs on its own goroutine over the slice it was handed and
// holds no lock of the list's, so the list has to leave that slice alone.
func TestPhotoListLeavesTheScannedSliceAlone(t *testing.T) {
	t.Parallel()

	photos := testPhotos("a.jpg", "b.jpg", "c.jpg")
	scanned := slices.Clone(photos)

	pl := newPhotoList()
	pl.reset(photos)
	require.True(t, pl.removePhoto(filepath.Join("/photos", "a.jpg")))
	pl.removePhotos(map[string]bool{filepath.Join("/photos", "b.jpg"): true})

	assert.Equal(t, scanned, photos, "the slice the scan is reading must not move")
}

// A scan reports by its own index into the folder it was given, which a delete
// meanwhile shifts out from under it.
func TestPhotoListScanReportsAfterARemoval(t *testing.T) {
	t.Parallel()

	photos := testPhotos("a.jpg", "b.jpg", "c.jpg")
	pl := newPhotoList()
	gen := pl.reset(photos)
	pl.initMeta(photos, nil)
	require.True(t, pl.removePhoto(filepath.Join("/photos", "a.jpg")))

	t.Run("the meta lands on the photo the scan read", func(t *testing.T) {
		displayIdx, ok := pl.setLoadedMeta(2, imaging.LoadedMeta{Favorite: true}, gen)
		require.True(t, ok)
		assert.Equal(t, 1, displayIdx)
		assert.False(t, pl.GetMeta(0).Favorite, "b.jpg was not the photo read")
		assert.True(t, pl.GetMeta(1).Favorite, "c.jpg was")
	})

	t.Run("a photo that is gone takes no meta", func(t *testing.T) {
		displayIdx, ok := pl.setLoadedMeta(0, imaging.LoadedMeta{Favorite: true}, gen)
		assert.True(t, ok)
		assert.Equal(t, -1, displayIdx)
		assert.False(t, pl.GetMeta(0).Favorite, "b.jpg keeps what it had")
	})
}

func TestPhotoListFilteredRows(t *testing.T) {
	t.Parallel()

	photos := testPhotos("a.jpg", "b.jpg", "c.jpg")
	pl := newPhotoList()
	gen := pl.reset(photos)
	pl.initMeta(photos, model.ColorMap{"b.jpg": {model.ColorRed}})
	pl.ToggleColorFilter(model.ColorRed)
	pl.applyFilter()
	require.Equal(t, 1, pl.count())

	// the row draws the photo behind it, not a copy taken when the filter ran:
	// the folder scan is still on its way with the thumbnails and the ratings
	thumbnail := image.NewGray(image.Rect(0, 0, 2, 2))
	displayIndex, ok := pl.setLoadedMeta(1, imaging.LoadedMeta{Thumbnail: thumbnail, Favorite: true, Ratable: true}, gen)
	require.True(t, ok)
	require.Equal(t, 0, displayIndex)

	photo, meta, ok := pl.itemAt(0)
	require.True(t, ok)
	assert.Equal(t, "b.jpg", photo.Name)
	assert.Equal(t, thumbnail, meta.Thumbnail)
	assert.True(t, meta.Favorite)
	assert.Equal(t, meta, pl.GetMeta(0))
	assert.Equal(t, []model.Photo{photos[1]}, pl.FilteredPhotos())
	assert.Equal(t, []model.PhotoMeta{meta}, pl.FilteredMeta())
}

func TestPhotoListRatingWrittenByTheApp(t *testing.T) {
	t.Parallel()

	photos := testPhotos("a.jpg")
	pl := newPhotoList()
	gen := pl.reset(photos)
	pl.applyFilter()

	pl.setItemMeta(0, []model.ColorLabel{model.ColorRed}, true)

	// the scan read the file before the toggle wrote into it
	thumbnail := image.NewGray(image.Rect(0, 0, 2, 2))
	_, ok := pl.setLoadedMeta(0, imaging.LoadedMeta{Thumbnail: thumbnail}, gen)
	require.True(t, ok)

	meta := pl.GetMeta(0)
	assert.True(t, meta.Favorite, "the rating the app wrote is the one on disk")
	assert.Equal(t, thumbnail, meta.Thumbnail, "the thumbnail it carries is still wanted")
	assert.Equal(t, []model.ColorLabel{model.ColorRed}, meta.Colors)
}

func TestPhotoListColorChangeLeavesTheRatingToTheScan(t *testing.T) {
	t.Parallel()

	photos := testPhotos("a.jpg")
	pl := newPhotoList()
	gen := pl.reset(photos)
	pl.applyFilter()

	pl.setItemColors(0, []model.ColorLabel{model.ColorBlue})
	_, ok := pl.setLoadedMeta(0, imaging.LoadedMeta{Favorite: true}, gen)
	require.True(t, ok)

	meta := pl.GetMeta(0)
	assert.True(t, meta.Favorite)
	assert.Equal(t, []model.ColorLabel{model.ColorBlue}, meta.Colors)
}

// A filter that keeps nothing is a filter all the same: reading an empty list
// as "no filter" would put the whole folder back on screen and hand every photo
// in it to Delete All.
func TestPhotoListFilterMatchingNothing(t *testing.T) {
	t.Parallel()

	photos := testPhotos("a.jpg", "b.jpg")
	pl := newPhotoList()
	pl.reset(photos)
	pl.initMeta(photos, model.ColorMap{})
	pl.ToggleColorFilter(model.ColorRed)
	pl.applyFilter()

	assert.Equal(t, 0, pl.count())
	assert.Empty(t, pl.FilteredPhotos())
	assert.Empty(t, pl.FilteredMeta())
	_, ok := pl.photoAt(0)
	assert.False(t, ok)
	_, _, ok = pl.itemAt(0)
	assert.False(t, ok)
	assert.Equal(t, -1, pl.displayIndex(0))

	active, _, count := pl.bulkState()
	assert.True(t, active)
	assert.Equal(t, 0, count)
}
