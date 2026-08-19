package ui

import (
	"errors"
	"fmt"
	"testing"
	"time"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"photo/internal/core/model"
	"photo/internal/core/tags"
)

func newTestTagsDialog(t *testing.T, callbacks TagsDialogCallbacks) *TagsDialog {
	t.Helper()
	return newTestTagsDialogWith(t, TagsDialogOptions{Filename: "DSC001.JPG", Date: defaultTestDate}, callbacks)
}

func newTestTagsDialogWith(t *testing.T, opts TagsDialogOptions, callbacks TagsDialogCallbacks) *TagsDialog {
	t.Helper()
	window := test.NewTempWindow(t, nil)
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

// The Fyne test driver keeps global canvas and font state, so these tests share
// a window and must not run in parallel.
func TestTagsDialog(t *testing.T) {
	t.Run("starts with nothing but the inputs", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		assert.True(t, d.copyTitleBtn.Disabled())
		assert.True(t, d.copyKeywordsBtn.Disabled())
		assert.False(t, d.resultBox.Visible())
		assert.False(t, d.status.Visible())
		assert.False(t, d.existingBox.Visible())
		assert.False(t, d.pathRow.Visible())
		assert.False(t, d.progress.Visible())
	})

	t.Run("shows generated tags and reports readiness", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		generated := model.Tags{Title: "A calm morning by the lake.", Keywords: fullKeywords()}

		d.SetTags(generated)

		assert.Equal(t, generated.Title, d.title.Text)
		assert.Equal(t, generated.KeywordLine(), d.keywords.Text)
		assert.Equal(t, generated, d.Tags())
		assert.False(t, d.copyTitleBtn.Disabled())
		assert.False(t, d.copyKeywordsBtn.Disabled())
		assert.True(t, d.resultBox.Visible())
		assert.True(t, d.status.Visible())
		assert.Contains(t, d.status.Text, "50 keywords")
		assert.False(t, d.progress.Visible())
		assert.Equal(t, "Regenerate", d.generateBtn.Text)
	})

	t.Run("reports problems while the user edits", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.SetTags(model.Tags{Title: "A calm morning.", Keywords: fullKeywords()})

		d.keywords.SetText("lake, forest")

		assert.Contains(t, d.status.Text, "2 keywords, expected 50")
		assert.False(t, d.copyKeywordsBtn.Disabled())
	})

	t.Run("disables copy once everything is cleared", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.SetTags(model.Tags{Title: "Title", Keywords: fullKeywords()})

		d.title.SetText("")
		d.keywords.SetText("")

		assert.True(t, d.copyTitleBtn.Disabled())
		assert.True(t, d.copyKeywordsBtn.Disabled())
		assert.Empty(t, d.status.Text)
		assert.False(t, d.status.Visible())
	})

	t.Run("copies each half on its own", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.SetTags(model.Tags{Title: "A calm morning.", Keywords: fullKeywords()})

		d.title.SetText("")

		assert.True(t, d.copyTitleBtn.Disabled())
		assert.False(t, d.copyKeywordsBtn.Disabled())
	})

	t.Run("offers no saving when there is nothing to save into", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		assert.Nil(t, d.saveSidecarBtn)
		assert.Nil(t, d.saveJPEGBtn)
	})

	t.Run("saves to the sidecar of the RAW pair and to the JPEG", func(t *testing.T) {
		sidecarCalls, jpegCalls := 0, 0
		d := newTestTagsDialogWith(t,
			TagsDialogOptions{Filename: "DSC001.JPG", HasRAW: true, IsJPEG: true},
			TagsDialogCallbacks{
				OnSaveSidecar: func() { sidecarCalls++ },
				OnSaveJPEG:    func() { jpegCalls++ },
			})
		require.NotNil(t, d.saveSidecarBtn)
		require.NotNil(t, d.saveJPEGBtn)
		assert.True(t, d.saveSidecarBtn.Disabled())
		assert.True(t, d.saveJPEGBtn.Disabled())

		d.SetTags(model.Tags{Title: "A calm morning.", Keywords: fullKeywords()})
		test.Tap(d.saveSidecarBtn)
		test.Tap(d.saveJPEGBtn)

		assert.Equal(t, 1, sidecarCalls)
		assert.Equal(t, 1, jpegCalls)
	})

	t.Run("saving needs a title or keywords", func(t *testing.T) {
		d := newTestTagsDialogWith(t,
			TagsDialogOptions{Filename: "DSC001.JPG", IsJPEG: true}, TagsDialogCallbacks{})
		d.SetTags(model.Tags{Title: "A calm morning.", Keywords: fullKeywords()})
		assert.False(t, d.saveJPEGBtn.Disabled())

		d.title.SetText("")
		assert.False(t, d.saveJPEGBtn.Disabled())

		d.keywords.SetText("")
		assert.True(t, d.saveJPEGBtn.Disabled())
	})

	t.Run("generate disables the button and runs the callback", func(t *testing.T) {
		calls := 0
		d := newTestTagsDialog(t, TagsDialogCallbacks{OnGenerate: func() { calls++ }})

		test.Tap(d.generateBtn)

		assert.Equal(t, 1, calls)
		assert.True(t, d.generateBtn.Disabled())
		assert.True(t, d.progress.Visible())
	})

	t.Run("failure re-enables generate and keeps the path entry hidden", func(t *testing.T) {
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
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.Fail(fmt.Errorf("looking up: %w", tags.ErrClaudeNotFound))

		assert.True(t, d.pathRow.Visible())
	})

	t.Run("trims the configured path", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.pathEntry.SetText("  /usr/local/bin/claude \n")

		assert.Equal(t, "/usr/local/bin/claude", d.ClaudePath())
	})

	t.Run("sends no notes when nothing is filled in", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		assert.Empty(t, d.Notes())
		assert.False(t, d.dateRow.Visible())
	})

	t.Run("renders concept and location in the prompt input format", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.concept.SetText("  slow travel  ")
		d.location.SetText(" Prague, Czechia ")

		assert.Equal(t, "Concept: slow travel\nLocation: Prague, Czechia", d.Notes())
	})

	t.Run("editorial reveals the date and adds the date line", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.location.SetText("Prague, Czechia")

		d.editorial.SetChecked(true)

		assert.True(t, d.dateRow.Visible())
		require.NotNil(t, d.date.Date)
		assert.Equal(t, defaultTestDate, *d.date.Date)
		assert.Equal(t, "Location: Prague, Czechia\nEditorial: August 18, 2026", d.Notes())
	})

	t.Run("editorial without a date stays a bare flag", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.editorial.SetChecked(true)

		d.date.SetDate(nil)

		assert.Equal(t, "Editorial:", d.Notes())
	})

	t.Run("unchecking editorial hides the date and drops the line", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.editorial.SetChecked(true)

		d.editorial.SetChecked(false)

		assert.False(t, d.dateRow.Visible())
		assert.Empty(t, d.Notes())
	})

	t.Run("shooting date replaces the default date", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.SetPhotoInfo(model.Tags{}, time.Date(2024, time.June, 13, 10, 0, 0, 0, time.UTC))

		d.editorial.SetChecked(true)
		assert.Equal(t, "Editorial: June 13, 2024", d.Notes())
		assert.False(t, d.existingBox.Visible())
		assert.Empty(t, d.existing.Text)
	})

	t.Run("keeps the existing block hidden for a file without tags", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.SetPhotoInfo(model.Tags{Title: "  ", Keywords: nil}, time.Time{})

		assert.False(t, d.existingBox.Visible())
	})

	t.Run("shows only the half the file carries", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.SetPhotoInfo(model.Tags{Keywords: []string{"lake"}}, time.Time{})

		assert.True(t, d.existingBox.Visible())
		assert.Equal(t, "lake", d.existing.Text)
	})

	t.Run("shooting date does not overwrite a typed date", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		picked := time.Date(2020, time.May, 1, 0, 0, 0, 0, time.UTC)
		d.date.SetDate(&picked)

		d.SetPhotoInfo(model.Tags{}, time.Date(2024, time.June, 13, 10, 0, 0, 0, time.UTC))

		require.NotNil(t, d.date.Date)
		assert.Equal(t, picked, *d.date.Date)
	})

	t.Run("keeps a cleared date cleared", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.date.SetDate(nil)

		d.SetPhotoInfo(model.Tags{}, time.Date(2024, time.June, 13, 10, 0, 0, 0, time.UTC))

		assert.Nil(t, d.date.Date)
	})

	t.Run("shows tags already written to the file", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.SetPhotoInfo(model.Tags{Title: "Old title.", Keywords: []string{"lake", "forest"}}, time.Time{})

		assert.True(t, d.existingBox.Visible())
		assert.Equal(t, "Old title.\nlake, forest", d.existing.Text)
		assert.Empty(t, d.title.Text, "existing tags must not overwrite the editable fields")
		assert.Empty(t, d.keywords.Text)
	})

	t.Run("copy reports the current, edited tags", func(t *testing.T) {
		var copiedTitle string
		var copiedKeywords []string
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.callbacks.OnCopyTitle = func() { copiedTitle = d.Tags().Title }
		d.callbacks.OnCopyKeywords = func() { copiedKeywords = d.Tags().Keywords }
		d.SetTags(model.Tags{Title: "Original", Keywords: fullKeywords()})

		d.title.SetText("Edited")
		d.keywords.SetText("lake,  forest ,")
		test.Tap(d.copyTitleBtn)
		test.Tap(d.copyKeywordsBtn)

		assert.Equal(t, "Edited", copiedTitle)
		assert.Equal(t, []string{"lake", "forest"}, copiedKeywords)
	})

	t.Run("escape closes the dialog from any input", func(t *testing.T) {
		inputs := map[string]func(*TagsDialog) fyne.Focusable{
			"concept":   func(d *TagsDialog) fyne.Focusable { return d.concept },
			"date":      func(d *TagsDialog) fyne.Focusable { return d.date },
			"editorial": func(d *TagsDialog) fyne.Focusable { return d.editorial },
			"keywords":  func(d *TagsDialog) fyne.Focusable { return d.keywords },
		}

		for name, input := range inputs {
			t.Run(name, func(t *testing.T) {
				closes := 0
				d := newTestTagsDialog(t, TagsDialogCallbacks{OnClose: func() { closes++ }})

				input(d).TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})

				assert.Equal(t, 1, closes)
			})
		}
	})

	t.Run("other keys still reach the input", func(t *testing.T) {
		closes := 0
		d := newTestTagsDialog(t, TagsDialogCallbacks{OnClose: func() { closes++ }})
		d.concept.SetText("castle")
		d.concept.CursorColumn = len("castle")

		d.concept.TypedKey(&fyne.KeyEvent{Name: fyne.KeyBackspace})

		assert.Equal(t, 0, closes)
		assert.Equal(t, "castl", d.concept.Text)
	})

	t.Run("close is reported once", func(t *testing.T) {
		closes := 0
		d := newTestTagsDialog(t, TagsDialogCallbacks{OnClose: func() { closes++ }})
		d.Show()

		test.Tap(d.closeBtn)
		d.Hide()

		assert.Equal(t, 1, closes)
	})

	t.Run("closing the dialog window reports close", func(t *testing.T) {
		closes := 0
		d := newTestTagsDialog(t, TagsDialogCallbacks{OnClose: func() { closes++ }})
		d.Show()

		d.dialog.Hide()

		assert.Equal(t, 1, closes)
	})
}

func TestNotesStayASCII(t *testing.T) {
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
