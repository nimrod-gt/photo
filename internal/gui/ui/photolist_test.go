package ui

import (
	"image"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"photo/internal/core/model"
)

func testPhotos(names ...string) []model.Photo {
	photos := make([]model.Photo, 0, len(names))
	for _, name := range names {
		photos = append(photos, model.Photo{ImagePath: filepath.Join("/photos", name), Name: name})
	}
	return photos
}

func TestPhotoListFilteredRows(t *testing.T) {
	t.Parallel()

	photos := testPhotos("a.jpg", "b.jpg", "c.jpg")
	pl := newPhotoList()
	gen := pl.reset(photos)
	pl.initMeta(photos, model.ColorMap{"b.jpg": {model.ColorRed}})
	pl.toggleColorFilter(model.ColorRed)
	pl.applyFilter()
	require.Equal(t, 1, pl.count())

	// the row draws the photo behind it, not a copy taken when the filter ran:
	// the folder scan is still on its way with the thumbnails and the ratings
	thumbnail := image.NewGray(image.Rect(0, 0, 2, 2))
	displayIndex, ok := pl.setLoadedMeta(1, thumbnail, true, gen)
	require.True(t, ok)
	require.Equal(t, 0, displayIndex)

	photo, meta, ok := pl.itemAt(0)
	require.True(t, ok)
	assert.Equal(t, "b.jpg", photo.Name)
	assert.Equal(t, thumbnail, meta.Thumbnail)
	assert.True(t, meta.Favorite)
	assert.Equal(t, meta, pl.metaAt(0))
	assert.Equal(t, []model.Photo{photos[1]}, pl.filteredPhotos())
	assert.Equal(t, []model.PhotoMeta{meta}, pl.filteredMeta())
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
	_, ok := pl.setLoadedMeta(0, thumbnail, false, gen)
	require.True(t, ok)

	meta := pl.metaAt(0)
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
	_, ok := pl.setLoadedMeta(0, nil, true, gen)
	require.True(t, ok)

	meta := pl.metaAt(0)
	assert.True(t, meta.Favorite)
	assert.Equal(t, []model.ColorLabel{model.ColorBlue}, meta.Colors)
}
