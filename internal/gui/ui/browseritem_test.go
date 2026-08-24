package ui

import (
	"image"
	"image/color"
	"testing"
	"time"

	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"photo/internal/core/model"
)

func TestBrowserItemColorDotsClearOnReuse(t *testing.T) {
	t.Parallel()

	dots := newColorDots()
	updateColorDots(dots, []model.ColorLabel{model.ColorRed, model.ColorBlue})
	require.True(t, dots.Objects[0].Visible())
	require.True(t, dots.Objects[1].Visible())
	require.False(t, dots.Objects[2].Visible())
	assert.Equal(t, colorLabelToColor(model.ColorRed), dots.Objects[0].(*canvas.Text).Color)
	assert.Equal(t, colorLabelToColor(model.ColorBlue), dots.Objects[1].(*canvas.Text).Color)

	// the list reuses the row for another photo with fewer colours
	updateColorDots(dots, []model.ColorLabel{model.ColorGreen})
	assert.True(t, dots.Objects[0].Visible())
	assert.Equal(t, colorLabelToColor(model.ColorGreen), dots.Objects[0].(*canvas.Text).Color)
	assert.False(t, dots.Objects[1].Visible())
	assert.False(t, dots.Objects[2].Visible())
}

func TestBrowserItemZeroDateClearsText(t *testing.T) {
	t.Parallel()

	dateText := canvas.NewText("stale", color.Black)
	updateDateText(dateText, time.Date(2026, 8, 24, 10, 30, 0, 0, time.UTC))
	assert.Equal(t, "2026-08-24 10:30", dateText.Text)

	updateDateText(dateText, time.Time{})
	assert.Empty(t, dateText.Text)
}

func TestBrowserItemStarFollowsFavorite(t *testing.T) {
	t.Parallel()

	star := canvas.NewText(favoriteMark, favoriteColor)
	star.Hide()

	updateStar(star, true)
	assert.True(t, star.Visible())

	updateStar(star, false)
	assert.False(t, star.Visible())
}

// The Fyne test driver keeps global canvas and font state, so a widget test
// that needs a window must not run in parallel.
func TestBrowserItemCarriesItsOwnState(t *testing.T) {
	item := newBrowserItem()
	test.NewTempWindow(t, item)

	assert.Positive(t, item.MinSize().Width)
	assert.Positive(t, item.MinSize().Height)

	favorite := model.Photo{Name: "DSC001.JPG"}
	favoriteMeta := model.PhotoMeta{
		Date:      time.Date(2026, 8, 24, 10, 30, 0, 0, time.UTC),
		Colors:    []model.ColorLabel{model.ColorRed, model.ColorBlue},
		Favorite:  true,
		Thumbnail: image.NewNRGBA(image.Rect(0, 0, 2, 2)),
	}
	item.update(favorite, favoriteMeta, favoriteMeta.Thumbnail)

	assert.Equal(t, "DSC001.JPG", item.name.Text)
	assert.True(t, item.star.Visible())
	assert.True(t, item.dots.Objects[0].Visible())
	assert.True(t, item.dots.Objects[1].Visible())
	assert.Equal(t, "2026-08-24 10:30", item.dateText.Text)
	assert.True(t, item.thumb.Visible())

	// the list reuses the same row for a plain photo
	item.update(model.Photo{Name: "DSC002.JPG"}, model.PhotoMeta{}, nil)

	assert.Equal(t, "DSC002.JPG", item.name.Text)
	assert.False(t, item.star.Visible())
	assert.False(t, item.dots.Objects[0].Visible())
	assert.False(t, item.dots.Objects[1].Visible())
	assert.Empty(t, item.dateText.Text)
	assert.False(t, item.thumb.Visible())
	assert.Nil(t, item.thumb.Image)
}
