package ui

import (
	"errors"
	"fmt"
	"testing"
	"time"
	"unicode"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"photo/internal/core/model"
	"photo/internal/core/tags"
)

func newTestTagsDialog(t *testing.T, callbacks TagsDialogCallbacks) *TagsDialog {
	t.Helper()
	window := test.NewTempWindow(t, nil)
	opts := TagsDialogOptions{Filename: "DSC001.JPG", Date: defaultTestDate}
	return NewTagsDialog(opts, window, callbacks)
}

var defaultTestDate = time.Date(2026, time.August, 18, 14, 30, 0, 0, time.UTC)

func fullKeywords() []string {
	keywords := make([]string, 0, model.KeywordCount)
	for i := range model.KeywordCount {
		keywords = append(keywords, fmt.Sprintf("keyword %d", i))
	}
	return keywords
}

func TestTagsDialog(t *testing.T) {
	t.Parallel()

	t.Run("starts with nothing but the inputs", func(t *testing.T) {
		t.Parallel()
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		assert.True(t, d.copyBtn.Disabled())
		assert.False(t, d.resultBox.Visible())
		assert.False(t, d.status.Visible())
		assert.False(t, d.existingBox.Visible())
		assert.False(t, d.pathRow.Visible())
		assert.False(t, d.progress.Visible())
	})

	t.Run("shows generated tags and reports readiness", func(t *testing.T) {
		t.Parallel()
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		generated := model.Tags{Title: "A calm morning by the lake.", Keywords: fullKeywords()}

		d.SetTags(generated)

		assert.Equal(t, generated.Title, d.title.Text)
		assert.Equal(t, generated.KeywordLine(), d.keywords.Text)
		assert.Equal(t, generated, d.Tags())
		assert.False(t, d.copyBtn.Disabled())
		assert.True(t, d.resultBox.Visible())
		assert.True(t, d.status.Visible())
		assert.Contains(t, d.status.Text, "50 keywords")
		assert.False(t, d.progress.Visible())
		assert.Equal(t, "Regenerate", d.generateBtn.Text)
	})

	t.Run("reports problems while the user edits", func(t *testing.T) {
		t.Parallel()
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.SetTags(model.Tags{Title: "A calm morning.", Keywords: fullKeywords()})

		d.keywords.SetText("lake, forest")

		assert.Contains(t, d.status.Text, "2 keywords, expected 50")
		assert.False(t, d.copyBtn.Disabled())
	})

	t.Run("disables copy once everything is cleared", func(t *testing.T) {
		t.Parallel()
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.SetTags(model.Tags{Title: "Title", Keywords: fullKeywords()})

		d.title.SetText("")
		d.keywords.SetText("")

		assert.True(t, d.copyBtn.Disabled())
		assert.Empty(t, d.status.Text)
		assert.False(t, d.status.Visible())
	})

	t.Run("generate disables the button and runs the callback", func(t *testing.T) {
		t.Parallel()
		calls := 0
		d := newTestTagsDialog(t, TagsDialogCallbacks{OnGenerate: func() { calls++ }})

		test.Tap(d.generateBtn)

		assert.Equal(t, 1, calls)
		assert.True(t, d.generateBtn.Disabled())
		assert.True(t, d.progress.Visible())
	})

	t.Run("failure re-enables generate and keeps the path entry hidden", func(t *testing.T) {
		t.Parallel()
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		test.Tap(d.generateBtn)

		d.Fail(errors.New("running claude: exit status 1"))

		assert.Equal(t, "running claude: exit status 1", d.status.Text)
		assert.True(t, d.status.Visible())
		assert.False(t, d.resultBox.Visible())
		assert.False(t, d.generateBtn.Disabled())
		assert.False(t, d.progress.Visible())
		assert.False(t, d.pathRow.Visible())
	})

	t.Run("missing binary reveals the path entry", func(t *testing.T) {
		t.Parallel()
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.Fail(fmt.Errorf("looking up: %w", tags.ErrClaudeNotFound))

		assert.True(t, d.pathRow.Visible())
	})

	t.Run("trims the configured path", func(t *testing.T) {
		t.Parallel()
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.pathEntry.SetText("  /usr/local/bin/claude \n")

		assert.Equal(t, "/usr/local/bin/claude", d.ClaudePath())
	})

	t.Run("sends no notes when nothing is filled in", func(t *testing.T) {
		t.Parallel()
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		assert.Empty(t, d.Notes())
		assert.False(t, d.dateRow.Visible())
	})

	t.Run("renders concept and location in the prompt input format", func(t *testing.T) {
		t.Parallel()
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.concept.SetText("  slow travel  ")
		d.location.SetText(" Prague, Czechia ")

		assert.Equal(t, "Concept: slow travel\nLocation: Prague, Czechia", d.Notes())
	})

	t.Run("editorial reveals the date and adds the date line", func(t *testing.T) {
		t.Parallel()
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.location.SetText("Prague, Czechia")

		d.editorial.SetChecked(true)

		assert.True(t, d.dateRow.Visible())
		require.NotNil(t, d.date.Date)
		assert.Equal(t, defaultTestDate, *d.date.Date)
		assert.Equal(t, "Location: Prague, Czechia\nEditorial: August 18, 2026", d.Notes())
	})

	t.Run("editorial without a date stays a bare flag", func(t *testing.T) {
		t.Parallel()
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.editorial.SetChecked(true)

		d.date.SetDate(nil)

		assert.Equal(t, "Editorial:", d.Notes())
	})

	t.Run("unchecking editorial hides the date and drops the line", func(t *testing.T) {
		t.Parallel()
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.editorial.SetChecked(true)

		d.editorial.SetChecked(false)

		assert.False(t, d.dateRow.Visible())
		assert.Empty(t, d.Notes())
	})

	t.Run("shooting date replaces the default date", func(t *testing.T) {
		t.Parallel()
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.SetPhotoInfo(model.Tags{}, time.Date(2024, time.June, 13, 10, 0, 0, 0, time.UTC))

		d.editorial.SetChecked(true)
		assert.Equal(t, "Editorial: June 13, 2024", d.Notes())
		assert.False(t, d.existingBox.Visible())
		assert.Empty(t, d.existing.Text)
	})

	t.Run("keeps the existing block hidden for a file without tags", func(t *testing.T) {
		t.Parallel()
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.SetPhotoInfo(model.Tags{Title: "  ", Keywords: nil}, time.Time{})

		assert.False(t, d.existingBox.Visible())
	})

	t.Run("shows only the half the file carries", func(t *testing.T) {
		t.Parallel()
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.SetPhotoInfo(model.Tags{Keywords: []string{"lake"}}, time.Time{})

		assert.True(t, d.existingBox.Visible())
		assert.Equal(t, "lake", d.existing.Text)
	})

	t.Run("shooting date does not overwrite a typed date", func(t *testing.T) {
		t.Parallel()
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		picked := time.Date(2020, time.May, 1, 0, 0, 0, 0, time.UTC)
		d.date.SetDate(&picked)

		d.SetPhotoInfo(model.Tags{}, time.Date(2024, time.June, 13, 10, 0, 0, 0, time.UTC))

		require.NotNil(t, d.date.Date)
		assert.Equal(t, picked, *d.date.Date)
	})

	t.Run("keeps a cleared date cleared", func(t *testing.T) {
		t.Parallel()
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.date.SetDate(nil)

		d.SetPhotoInfo(model.Tags{}, time.Date(2024, time.June, 13, 10, 0, 0, 0, time.UTC))

		assert.Nil(t, d.date.Date)
	})

	t.Run("shows tags already written to the file", func(t *testing.T) {
		t.Parallel()
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.SetPhotoInfo(model.Tags{Title: "Old title.", Keywords: []string{"lake", "forest"}}, time.Time{})

		assert.True(t, d.existingBox.Visible())
		assert.Equal(t, "Old title.\nlake, forest", d.existing.Text)
		assert.Empty(t, d.title.Text, "existing tags must not overwrite the editable fields")
		assert.Empty(t, d.keywords.Text)
	})

	t.Run("copy reports the current, edited tags", func(t *testing.T) {
		t.Parallel()
		var copied model.Tags
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.callbacks.OnCopy = func() { copied = d.Tags() }
		d.SetTags(model.Tags{Title: "Original", Keywords: fullKeywords()})

		d.title.SetText("Edited")
		d.keywords.SetText("lake,  forest ,")
		test.Tap(d.copyBtn)

		assert.Equal(t, model.Tags{Title: "Edited", Keywords: []string{"lake", "forest"}}, copied)
	})

	t.Run("close is reported once", func(t *testing.T) {
		t.Parallel()
		closes := 0
		d := newTestTagsDialog(t, TagsDialogCallbacks{OnClose: func() { closes++ }})
		d.Show()

		test.Tap(d.closeBtn)
		d.Hide()

		assert.Equal(t, 1, closes)
	})

	t.Run("closing the dialog window reports close", func(t *testing.T) {
		t.Parallel()
		closes := 0
		d := newTestTagsDialog(t, TagsDialogCallbacks{OnClose: func() { closes++ }})
		d.Show()

		d.dialog.Hide()

		assert.Equal(t, 1, closes)
	})
}

func TestNotesStayASCII(t *testing.T) {
	t.Parallel()
	d := newTestTagsDialog(t, TagsDialogCallbacks{})
	d.concept.SetText("slow travel")
	d.location.SetText("Prague, Czechia")
	d.editorial.SetChecked(true)

	notes := d.Notes()
	require.NotEmpty(t, notes)
	for _, r := range notes {
		assert.LessOrEqual(t, r, rune(unicode.MaxASCII), "notes must stay ASCII so they match the prompt")
	}
}
