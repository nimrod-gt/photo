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
	bar.Container().Resize(fyne.NewSize(viewerWidth, viewerHeight))
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

		assert.True(t, bar.title.content.Visible())
		assert.True(t, bar.keywords.content.Visible())
		assert.Equal(t, testTags.Title, bar.title.label.Text)
		assert.Equal(t, "sunset, harbour, boat", bar.keywords.label.Text)
	})

	t.Run("stays hidden for a photo without tags", func(t *testing.T) {
		bar := newTestTagBar(t)
		bar.SetTags(model.Tags{})

		assert.False(t, bar.title.content.Visible())
		assert.False(t, bar.keywords.content.Visible())
	})

	t.Run("hides the title alone when only keywords are known", func(t *testing.T) {
		bar := newTestTagBar(t)
		bar.SetTags(model.Tags{Keywords: []string{"sunset"}})

		assert.False(t, bar.title.content.Visible())
		assert.True(t, bar.keywords.content.Visible())
	})

	t.Run("clearing takes it away", func(t *testing.T) {
		bar := newTestTagBar(t)
		bar.SetTags(testTags)
		bar.Clear()

		assert.False(t, bar.title.content.Visible())
		assert.False(t, bar.keywords.content.Visible())
	})

	t.Run("disabled bar stays away and comes back on its own", func(t *testing.T) {
		bar := newTestTagBar(t)
		bar.SetEnabled(false)
		bar.SetTags(testTags)

		assert.False(t, bar.title.content.Visible())

		bar.SetEnabled(true)

		assert.True(t, bar.title.content.Visible())
	})

	t.Run("suppression wins over the user toggle", func(t *testing.T) {
		bar := newTestTagBar(t)
		bar.SetTags(testTags)
		bar.SetSuppressed(true)

		assert.False(t, bar.title.content.Visible())

		bar.SetEnabled(true)

		assert.False(t, bar.title.content.Visible())

		bar.SetSuppressed(false)

		assert.True(t, bar.title.content.Visible())
	})

	t.Run("hides a title that is nothing but spaces", func(t *testing.T) {
		bar := newTestTagBar(t)
		bar.SetTags(model.Tags{Title: "   ", Keywords: []string{"sunset"}})

		assert.False(t, bar.title.content.Visible())
		assert.True(t, bar.keywords.content.Visible())
	})

	t.Run("each row is a plate of its own width", func(t *testing.T) {
		bar := newTestTagBar(t)
		bar.SetTags(model.Tags{Title: "Boat", Keywords: testTags.Keywords})

		assert.Less(t, bar.title.content.Size().Width, bar.keywords.content.Size().Width)

		expected := fyne.MeasureText("Boat", theme.TextSize(), bar.title.label.TextStyle).Width
		assert.InDelta(t, expected+2*tagBarTextPadX, bar.title.content.Size().Width, 0.01)
	})

	t.Run("the rows sit one above the other with a gap", func(t *testing.T) {
		bar := newTestTagBar(t)
		bar.SetTags(testTags)

		title, keywords := bar.title.content, bar.keywords.content
		assert.Equal(t, viewerHeight-tagBarMargin, keywords.Position().Y+keywords.Size().Height)
		assert.Equal(t, keywords.Position().Y-tagBarRowGap, title.Position().Y+title.Size().Height)
		assert.Equal(t, tagBarMargin, title.Position().X)
		assert.Equal(t, tagBarMargin, keywords.Position().X)
	})

	t.Run("the keyword row takes the bottom when the title is hidden", func(t *testing.T) {
		bar := newTestTagBar(t)
		bar.SetTags(model.Tags{Keywords: testTags.Keywords})

		keywords := bar.keywords.content
		assert.Equal(t, viewerHeight-tagBarMargin, keywords.Position().Y+keywords.Size().Height)
	})

	t.Run("the title is capped narrower than the keywords", func(t *testing.T) {
		bar := newTestTagBar(t)
		long := strings.Repeat("a very long line ", 40)
		bar.SetTags(model.Tags{Title: long, Keywords: []string{long}})

		assert.Equal(t, viewerWidth*tagBarTitleWidthPct, bar.title.content.Size().Width)
		assert.Equal(t, viewerWidth*tagBarMaxWidthPct, bar.keywords.content.Size().Width)
	})

	t.Run("the rows are remeasured for the next photo, without a resize", func(t *testing.T) {
		bar := newTestTagBar(t)
		bar.SetTags(model.Tags{Title: "Boat"})
		short := bar.title.content.Size().Width

		bar.SetTags(model.Tags{Title: "Boat at the end of a long summer day"})
		assert.Less(t, short, bar.title.content.Size().Width)

		bar.SetTags(model.Tags{Title: "Boat"})
		assert.Equal(t, short, bar.title.content.Size().Width)
	})

	t.Run("a viewer too narrow for the margins is not given a negative size", func(t *testing.T) {
		bar := newTestTagBar(t)
		bar.SetTags(testTags)
		bar.Container().Resize(fyne.NewSize(10, 10))

		assert.GreaterOrEqual(t, bar.title.content.Size().Width, float32(0))
		assert.GreaterOrEqual(t, bar.title.content.Position().Y, float32(0))
	})
}
