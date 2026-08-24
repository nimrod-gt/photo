package ui

import (
	"image/color"
	"testing"
	"time"

	"fyne.io/fyne/v2/canvas"
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
