package ui

import (
	"image"
	"testing"

	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
)

// The Fyne test driver keeps global canvas and font state, so a widget test
// that needs a window must not run in parallel.
func TestGridItemCarriesItsOwnState(t *testing.T) {
	item := newGridItem(200)
	test.NewTempWindow(t, item)

	assert.Positive(t, item.MinSize().Width)
	assert.Positive(t, item.MinSize().Height)
	// the tile is the one place the GPU does the scaling; a CatmullRom pass per
	// repaint here is what made a loading row stutter
	assert.Equal(t, canvas.ImageScaleFastest, item.thumb.ScaleMode)

	thumbnail := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	item.update("DSC001.JPG", thumbnail)

	assert.Equal(t, "DSC001.JPG", item.name.Text)
	assert.Equal(t, thumbnail, item.thumb.Image)
	assert.True(t, item.thumb.Visible())

	// the grid reuses the same tile for a photo without a thumbnail yet
	item.update("DSC002.JPG", nil)

	assert.Equal(t, "DSC002.JPG", item.name.Text)
	assert.Nil(t, item.thumb.Image)
	assert.False(t, item.thumb.Visible())
}

func TestGridItemHidesThumbWithoutImageOnFirstUpdate(t *testing.T) {
	item := newGridItem(200)
	test.NewTempWindow(t, item)

	item.update("DSC001.JPG", nil)

	assert.Nil(t, item.thumb.Image)
	assert.False(t, item.thumb.Visible())
}
