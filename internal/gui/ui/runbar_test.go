package ui

import (
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRunBar(t *testing.T) *RunBar {
	t.Helper()
	bar := NewRunBar()
	test.NewTempWindow(t, bar.Container())
	bar.Container().Resize(fyne.NewSize(viewerWidth, viewerHeight))
	// The test driver runs fyne.Do on the caller's goroutine, so a tick left
	// armed relabels a plate from the timer goroutine while another test draws
	// text on its own - and the text shaper is not shared that way.
	t.Cleanup(func() { bar.SetRuns(nil) })
	return bar
}

func runBarTexts(bar *RunBar) []string {
	texts := make([]string, 0, len(bar.layout.rows))
	for _, row := range bar.layout.rows {
		texts = append(texts, row.label.Text)
	}
	return texts
}

// The Fyne test driver keeps global canvas and font state, so these tests share
// a window and must not run in parallel.
func TestRunBar(t *testing.T) {
	t.Run("stays away while nothing runs", func(t *testing.T) {
		bar := newTestRunBar(t)

		assert.False(t, bar.Container().Visible())
		assert.Empty(t, bar.layout.rows)
	})

	t.Run("shows a plate per run", func(t *testing.T) {
		bar := newTestRunBar(t)
		bar.SetRuns([]RunItem{
			{Name: "DSC001.JPG", Since: time.Now().Add(-72 * time.Second)},
			{Name: "DSC002.JPG", Since: time.Now()},
		})

		assert.True(t, bar.Container().Visible())
		require.Len(t, bar.layout.rows, 2)
		assert.Equal(t, []string{"1:12  DSC001.JPG", "0:00  DSC002.JPG"}, runBarTexts(bar))
	})

	t.Run("counts the hours of a long run", func(t *testing.T) {
		bar := newTestRunBar(t)
		bar.SetRuns([]RunItem{{Name: "DSC001.JPG", Since: time.Now().Add(-(time.Hour + 125*time.Second))}})

		assert.Equal(t, []string{"1:02:05  DSC001.JPG"}, runBarTexts(bar))
	})

	t.Run("takes a run without a start time as just begun", func(t *testing.T) {
		bar := newTestRunBar(t)
		bar.SetRuns([]RunItem{{Name: "DSC001.JPG"}})

		assert.Equal(t, []string{"0:00  DSC001.JPG"}, runBarTexts(bar))
	})

	t.Run("drops the plate of a run that ended", func(t *testing.T) {
		bar := newTestRunBar(t)
		bar.SetRuns([]RunItem{{Name: "DSC001.JPG"}, {Name: "DSC002.JPG"}})
		bar.SetRuns([]RunItem{{Name: "DSC002.JPG", Since: time.Now()}})

		require.Len(t, bar.layout.rows, 1)
		assert.Equal(t, []string{"0:00  DSC002.JPG"}, runBarTexts(bar))

		bar.SetRuns(nil)

		assert.False(t, bar.Container().Visible())
		assert.Empty(t, bar.layout.rows)
	})

	t.Run("stacks the plates in the top right corner", func(t *testing.T) {
		bar := newTestRunBar(t)
		bar.SetRuns([]RunItem{{Name: "DSC001.JPG", Since: time.Now()}, {Name: "DSC002.JPG", Since: time.Now()}})
		require.Len(t, bar.layout.rows, 2)

		first, second := bar.layout.rows[0].content, bar.layout.rows[1].content
		assert.Equal(t, tagBarMargin, first.Position().Y)
		assert.Greater(t, second.Position().Y, first.Position().Y)
		for _, row := range []fyne.CanvasObject{first, second} {
			assert.Equal(t, viewerWidth-tagBarMargin, row.Position().X+row.Size().Width)
			assert.Greater(t, row.Size().Width, float32(0))
		}
	})

	t.Run("gives a plate room for its whole line", func(t *testing.T) {
		bar := newTestRunBar(t)
		bar.SetRuns([]RunItem{{Name: "DSC05556.JPG", Since: time.Now()}})
		require.Len(t, bar.layout.rows, 1)

		row := bar.layout.rows[0]
		line := widget.NewLabel(row.label.Text)
		assert.GreaterOrEqual(t, row.content.Size().Width,
			line.MinSize().Width+theme.IconInlineSize()+theme.Padding(),
			"a plate cut to the width of its text truncates it")
	})

	t.Run("keeps the running time when the name does not fit", func(t *testing.T) {
		bar := newTestRunBar(t)
		bar.SetRuns([]RunItem{{Name: strings.Repeat("very-long-name", 20) + ".JPG", Since: time.Now()}})
		require.Len(t, bar.layout.rows, 1)

		assert.LessOrEqual(t, bar.layout.rows[0].content.Size().Width, viewerWidth*runBarWidthPct)
	})

	t.Run("ticks only while something runs", func(t *testing.T) {
		bar := newTestRunBar(t)
		assert.Nil(t, bar.timer)

		bar.SetRuns([]RunItem{{Name: "DSC001.JPG", Since: time.Now()}})
		assert.NotNil(t, bar.timer)

		bar.SetRuns(nil)
		assert.Nil(t, bar.timer)
	})
}
