package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"github.com/stretchr/testify/assert"

	"photo/internal/core/model"
)

func newTestTagBar(t *testing.T) *TagBar {
	t.Helper()
	bar := NewTagBar()
	test.NewTempWindow(t, bar.Container())
	return bar
}

const (
	viewerWidth  = float32(800)
	viewerHeight = float32(600)
)

var testTags = model.Tags{Title: "Sunset over the harbour", Keywords: []string{"sunset", "harbour", "boat"}}

// The Fyne test driver keeps global canvas and font state, so these tests share
// a window and must not run in parallel.
func TestTagBar(t *testing.T) {
	t.Run("shows the tags it is given", func(t *testing.T) {
		bar := newTestTagBar(t)
		bar.SetTags(testTags)

		assert.True(t, bar.content.Visible())
		assert.Equal(t, testTags.Title, bar.title.Text)
		assert.Equal(t, "sunset, harbour, boat", bar.keywords.Text)
	})

	t.Run("stays hidden for a photo without tags", func(t *testing.T) {
		bar := newTestTagBar(t)
		bar.SetTags(model.Tags{})

		assert.False(t, bar.content.Visible())
	})

	t.Run("hides the title alone when only keywords are known", func(t *testing.T) {
		bar := newTestTagBar(t)
		bar.SetTags(model.Tags{Keywords: []string{"sunset"}})

		assert.True(t, bar.content.Visible())
		assert.False(t, bar.title.Visible())
		assert.True(t, bar.keywords.Visible())
	})

	t.Run("clearing takes it away", func(t *testing.T) {
		bar := newTestTagBar(t)
		bar.SetTags(testTags)
		bar.Clear()

		assert.False(t, bar.content.Visible())
	})

	t.Run("disabled bar stays away and comes back on its own", func(t *testing.T) {
		bar := newTestTagBar(t)
		bar.SetEnabled(false)
		bar.SetTags(testTags)

		assert.False(t, bar.content.Visible())

		bar.SetEnabled(true)

		assert.True(t, bar.content.Visible())
	})

	t.Run("suppression wins over the user toggle", func(t *testing.T) {
		bar := newTestTagBar(t)
		bar.SetTags(testTags)
		bar.SetSuppressed(true)

		assert.False(t, bar.content.Visible())

		bar.SetEnabled(true)

		assert.False(t, bar.content.Visible())

		bar.SetSuppressed(false)

		assert.True(t, bar.content.Visible())
	})

	t.Run("hides a title that is nothing but spaces", func(t *testing.T) {
		bar := newTestTagBar(t)
		bar.SetTags(model.Tags{Title: "   ", Keywords: []string{"sunset"}})

		assert.True(t, bar.content.Visible())
		assert.False(t, bar.title.Visible())
	})

	t.Run("the overlay is as wide as its text", func(t *testing.T) {
		bar := newTestTagBar(t)
		bar.SetTags(testTags)
		bar.Container().Resize(fyne.NewSize(viewerWidth, viewerHeight))
		short := bar.content.Size().Width

		expected := fyne.MeasureText(testTags.Title, theme.TextSize(), bar.title.TextStyle).Width
		assert.InDelta(t, expected, short, float64(2*theme.InnerPadding()+2*theme.Padding()))
	})

	t.Run("a long title is truncated instead of stretching the overlay", func(t *testing.T) {
		bar := newTestTagBar(t)
		bar.SetTags(testTags)
		bar.Container().Resize(fyne.NewSize(viewerWidth, viewerHeight))
		short := bar.content.Size().Width

		bar.SetTags(model.Tags{Title: strings.Repeat("a very long title ", 40), Keywords: testTags.Keywords})
		bar.Container().Resize(fyne.NewSize(viewerWidth, viewerHeight+1))

		assert.Equal(t, viewerWidth*tagBarMaxWidthPct, bar.content.Size().Width)
		assert.Less(t, short, bar.content.Size().Width)
	})

	t.Run("a viewer too narrow for the margins is not given a negative size", func(t *testing.T) {
		bar := newTestTagBar(t)
		bar.SetTags(testTags)
		bar.Container().Resize(fyne.NewSize(10, 10))

		assert.GreaterOrEqual(t, bar.content.Size().Width, float32(0))
		assert.GreaterOrEqual(t, bar.content.Position().Y, float32(0))
	})
}
