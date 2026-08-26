package ui

import (
	"errors"
	"fmt"
	"testing"
	"time"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/go-gl/glfw/v3.4/glfw"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"photo/internal/core/claudebin"
	"photo/internal/core/model"
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

	t.Run("offers no saving when there is no JPEG to save into", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		assert.Nil(t, d.saveJPEGBtn)
	})

	t.Run("saves into the JPEG", func(t *testing.T) {
		jpegCalls := 0
		d := newTestTagsDialogWith(t,
			TagsDialogOptions{Filename: "DSC001.JPG", IsJPEG: true},
			TagsDialogCallbacks{OnSaveJPEG: func() { jpegCalls++ }})
		require.NotNil(t, d.saveJPEGBtn)
		assert.True(t, d.saveJPEGBtn.Disabled())

		d.SetTags(model.Tags{Title: "A calm morning.", Keywords: fullKeywords()})
		test.Tap(d.saveJPEGBtn)

		assert.Equal(t, 1, jpegCalls)
	})

	t.Run("saving waits for tags that meet every stock requirement", func(t *testing.T) {
		d := newTestTagsDialogWith(t,
			TagsDialogOptions{Filename: "DSC001.JPG", IsJPEG: true}, TagsDialogCallbacks{})
		ready := model.Tags{Title: "A calm morning.", Keywords: fullKeywords()}
		d.SetTags(ready)
		assert.False(t, d.saveJPEGBtn.Disabled())

		d.keywords.SetText("lake, forest")
		assert.True(t, d.saveJPEGBtn.Disabled(), "too few keywords to upload")

		d.keywords.SetText(ready.KeywordLine())
		d.title.SetText("")
		assert.True(t, d.saveJPEGBtn.Disabled(), "a photo without a title is not ready either")

		d.title.SetText("Cafe\u0301 at dawn.")
		assert.True(t, d.saveJPEGBtn.Disabled(), "the title leaves the characters stock sites accept")

		d.title.SetText(ready.Title)
		assert.False(t, d.saveJPEGBtn.Disabled())
	})

	t.Run("generate disables the button and runs the callback", func(t *testing.T) {
		calls := 0
		d := newTestTagsDialog(t, TagsDialogCallbacks{OnGenerate: func() { calls++ }})

		test.Tap(d.generateBtn)

		assert.Equal(t, 1, calls)
		assert.True(t, d.generateBtn.Disabled())
		assert.True(t, d.progress.Visible())
	})

	t.Run("a run leaves no way out but cancelling or backgrounding it", func(t *testing.T) {
		d := newTestTagsDialogWith(t,
			TagsDialogOptions{Filename: "DSC001.JPG", IsJPEG: true}, TagsDialogCallbacks{})
		assert.Equal(t,
			[]fyne.CanvasObject{d.closeBtn, d.copyTitleBtn, d.copyKeywordsBtn, d.saveJPEGBtn, d.generateBtn},
			d.buttons.Objects)

		test.Tap(d.generateBtn)

		assert.Equal(t,
			[]fyne.CanvasObject{d.cancelRunBtn, d.backgroundBtn, d.generateBtn},
			d.buttons.Objects)
	})

	t.Run("a finished run brings the other buttons back", func(t *testing.T) {
		for name, finish := range map[string]func(*TagsDialog){
			"tags":    func(d *TagsDialog) { d.SetTags(model.Tags{Title: "A calm morning.", Keywords: fullKeywords()}) },
			"failure": func(d *TagsDialog) { d.Fail(errors.New("running claude: exit status 1")) },
		} {
			t.Run(name, func(t *testing.T) {
				d := newTestTagsDialog(t, TagsDialogCallbacks{})
				test.Tap(d.generateBtn)

				finish(d)

				assert.Equal(t,
					[]fyne.CanvasObject{d.closeBtn, d.copyTitleBtn, d.copyKeywordsBtn, d.generateBtn},
					d.buttons.Objects)
			})
		}
	})

	t.Run("a reopened dialog catches up with the run it left going", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.Generating()

		assert.True(t, d.generateBtn.Disabled())
		assert.True(t, d.progress.Visible())
		assert.Contains(t, d.status.Text, "Generating")
		assert.Equal(t,
			[]fyne.CanvasObject{d.cancelRunBtn, d.backgroundBtn, d.generateBtn},
			d.buttons.Objects)
	})

	t.Run("a read landing mid-run fills the fields and leaves the status alone", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.Generating()

		existing := model.Tags{Title: "What the file already said.", Keywords: fullKeywords()}
		d.SetPhotoInfo(existing, defaultTestDate)

		assert.Equal(t, existing, d.Tags())
		assert.Contains(t, d.status.Text, "Generating")

		d.Fail(errors.New("running claude: exit status 1"))

		assert.Equal(t, existing, d.Tags(), "a failed run leaves what the file held on screen")
		assert.Equal(t, "running claude: exit status 1", d.status.Text)
	})

	t.Run("cancel and background report to the app", func(t *testing.T) {
		var cancels, backgrounds, closes int
		d := newTestTagsDialog(t, TagsDialogCallbacks{
			OnCancelRun:  func() { cancels++ },
			OnBackground: func() { backgrounds++ },
			OnClose:      func() { closes++ },
		})
		test.Tap(d.generateBtn)

		test.Tap(d.cancelRunBtn)
		test.Tap(d.backgroundBtn)

		assert.Equal(t, 1, cancels)
		assert.Equal(t, 1, backgrounds)
		assert.Equal(t, 0, closes, "the app decides what closing means for a run")
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

		d.Fail(fmt.Errorf("looking up: %w", claudebin.ErrNotFound))

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
		assert.False(t, d.resultBox.Visible())
	})

	t.Run("keeps the fields hidden for a file without tags", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.SetPhotoInfo(model.Tags{Title: "  ", Keywords: nil}, time.Time{})

		assert.False(t, d.resultBox.Visible())
		assert.Empty(t, d.title.Text)
	})

	t.Run("fills only the half the file carries", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.SetPhotoInfo(model.Tags{Keywords: []string{"lake"}}, time.Time{})

		assert.True(t, d.resultBox.Visible())
		assert.Empty(t, d.title.Text)
		assert.Equal(t, "lake", d.keywords.Text)
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

	t.Run("fills the fields with the tags already written to the file", func(t *testing.T) {
		d := newTestTagsDialogWith(t,
			TagsDialogOptions{Filename: "DSC001.JPG", IsJPEG: true}, TagsDialogCallbacks{})

		d.SetPhotoInfo(model.Tags{Title: "Old title.", Keywords: []string{"lake", "forest"}}, time.Time{})

		assert.True(t, d.resultBox.Visible())
		assert.Equal(t, "Old title.", d.title.Text)
		assert.Equal(t, "lake, forest", d.keywords.Text)
		assert.Equal(t, model.Tags{Title: "Old title.", Keywords: []string{"lake", "forest"}}, d.Tags())
		assert.True(t, d.saveJPEGBtn.Disabled(), "two keywords are not a stock upload")
		assert.True(t, d.status.Visible())
	})

	t.Run("existing tags do not overwrite a generated result", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.SetTags(model.Tags{Title: "A calm morning.", Keywords: fullKeywords()})

		d.SetPhotoInfo(model.Tags{Title: "Old title.", Keywords: []string{"lake"}}, time.Time{})

		assert.Equal(t, "A calm morning.", d.title.Text)
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

	t.Run("the typed location is what the tags carry", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.location.SetText("  Cascais, Portugal  ")

		assert.Equal(t, model.Place{Location: "Cascais, Portugal"}, d.Tags().Place)
	})

	t.Run("a generated split reaches the tags", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.location.SetText("Cascais, Portugal")

		d.SetTags(model.Tags{
			Title:    "A calm morning.",
			Keywords: fullKeywords(),
			Place: model.Place{
				Location: "Cascais, Portugal",
				City:     "Cascais",
				State:    "Lisboa",
				Country:  "Portugal",
			},
		})

		assert.Equal(t, model.Place{
			Location: "Cascais, Portugal",
			City:     "Cascais",
			State:    "Lisboa",
			Country:  "Portugal",
		}, d.Tags().Place)
	})

	t.Run("editing the location afterwards drops the split", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.location.SetText("Cascais, Portugal")
		d.SetTags(model.Tags{
			Title:    "A calm morning.",
			Keywords: fullKeywords(),
			Place:    model.Place{Location: "Cascais, Portugal", City: "Cascais", Country: "Portugal"},
		})

		d.location.SetText("Sintra, Portugal")

		assert.Equal(t, model.Place{Location: "Sintra, Portugal"}, d.Tags().Place,
			"a split the user cannot see must not outlive the location it was made from")
	})

	t.Run("a location typed during the run wins over the split", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.Generating()
		d.location.SetText("Sintra, Portugal")

		d.SetTags(model.Tags{
			Title:    "A calm morning.",
			Keywords: fullKeywords(),
			Place:    model.Place{Location: "Cascais, Portugal", City: "Cascais"},
		})

		assert.Equal(t, "Sintra, Portugal", d.location.Text)
		assert.Equal(t, model.Place{Location: "Sintra, Portugal"}, d.Tags().Place)
	})

	t.Run("a place read from the file fills the empty location entry", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.SetPhotoInfo(model.Tags{
			Title:    "Old title.",
			Keywords: []string{"lake"},
			Place:    model.Place{Location: "Cascais, Portugal", City: "Cascais", Country: "Portugal"},
		}, time.Time{})

		assert.Equal(t, "Cascais, Portugal", d.location.Text)
		assert.Equal(t, model.Place{Location: "Cascais, Portugal", City: "Cascais", Country: "Portugal"},
			d.Tags().Place)
	})

	// A sidecar another tool wrote can hold a location and no tags at all, and it
	// is still the location of this photo.
	t.Run("a place read from a file without tags fills the location entry", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.SetPhotoInfo(model.Tags{Place: model.Place{Location: "Cascais, Portugal", City: "Cascais"}}, time.Time{})

		assert.Equal(t, "Cascais, Portugal", d.location.Text)
		assert.Equal(t, model.Place{Location: "Cascais, Portugal", City: "Cascais"}, d.Tags().Place)
		assert.True(t, d.resultBox.Hidden, "there are no tags to show")
	})

	t.Run("a place read from the file leaves a typed location alone", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.location.SetText("Sintra, Portugal")

		d.SetPhotoInfo(model.Tags{
			Title:    "Old title.",
			Keywords: []string{"lake"},
			Place:    model.Place{Location: "Cascais, Portugal", City: "Cascais"},
		}, time.Time{})

		assert.Equal(t, "Sintra, Portugal", d.location.Text)
		assert.Equal(t, model.Place{Location: "Sintra, Portugal"}, d.Tags().Place)
	})

	// A Lightroom sidecar can carry photoshop:City without an
	// Iptc4xmpCore:Location, and nothing about it is stale.
	t.Run("a split without free text survives an untouched entry", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.SetPhotoInfo(model.Tags{
			Title:    "Old title.",
			Keywords: []string{"lake"},
			Place:    model.Place{City: "Cascais", Country: "Portugal"},
		}, time.Time{})

		assert.Empty(t, d.location.Text)
		assert.Equal(t, model.Place{City: "Cascais", Country: "Portugal"}, d.Tags().Place)
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

func TestTagsDialog_Escape(t *testing.T) {
	t.Run("an input hands Escape to the app", func(t *testing.T) {
		var escapes, closes int
		d := newTestTagsDialog(t, TagsDialogCallbacks{
			OnEscape: func() { escapes++ },
			OnClose:  func() { closes++ },
		})

		d.concept.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})

		assert.Equal(t, 1, escapes)
		assert.Equal(t, 0, closes, "the app decides whether the dialog closes")
	})

	t.Run("every input takes the same route", func(t *testing.T) {
		var escapes int
		d := newTestTagsDialog(t, TagsDialogCallbacks{OnEscape: func() { escapes++ }})

		for _, entry := range []*dialogEntry{d.concept, d.location, d.pathEntry, d.title, d.keywords} {
			entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
		}
		d.date.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
		d.editorial.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})

		assert.Equal(t, 7, escapes)
	})

	t.Run("Escape closes when the app registers no handler", func(t *testing.T) {
		var closes int
		d := newTestTagsDialog(t, TagsDialogCallbacks{OnClose: func() { closes++ }})

		d.concept.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})

		assert.Equal(t, 1, closes)
	})

	t.Run("the multi-line fields let Tab traverse the dialog", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		assert.False(t, d.title.AcceptsTab())
		assert.False(t, d.keywords.AcceptsTab())
	})

	t.Run("the Close button closes whatever else is on screen", func(t *testing.T) {
		var escapes, closes int
		d := newTestTagsDialog(t, TagsDialogCallbacks{
			OnEscape: func() { escapes++ },
			OnClose:  func() { closes++ },
		})

		test.Tap(d.closeBtn)

		assert.Equal(t, 1, closes)
		assert.Equal(t, 0, escapes)
	})
}

func TestTagsDialog_ButtonKeys(t *testing.T) {
	t.Run("a focused button answers Space and Enter", func(t *testing.T) {
		for name, key := range map[string]fyne.KeyName{
			"space":  fyne.KeySpace,
			"return": fyne.KeyReturn,
			"enter":  fyne.KeyEnter,
		} {
			t.Run(name, func(t *testing.T) {
				closes := 0
				d := newTestTagsDialog(t, TagsDialogCallbacks{OnClose: func() { closes++ }})

				d.closeBtn.TypedKey(&fyne.KeyEvent{Name: key})

				assert.Equal(t, 1, closes)
			})
		}
	})

	t.Run("a disabled button stays silent", func(t *testing.T) {
		copies := 0
		d := newTestTagsDialog(t, TagsDialogCallbacks{OnCopyTitle: func() { copies++ }})
		require.True(t, d.copyTitleBtn.Disabled())

		d.copyTitleBtn.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})

		assert.Equal(t, 0, copies)
	})

	t.Run("a button hands Escape to the app", func(t *testing.T) {
		escapes := 0
		d := newTestTagsDialog(t, TagsDialogCallbacks{OnEscape: func() { escapes++ }})

		d.generateBtn.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})

		assert.Equal(t, 1, escapes)
	})

	t.Run("the run is commanded from whatever button holds the focus", func(t *testing.T) {
		var cancels, backgrounds int
		d := newTestTagsDialog(t, TagsDialogCallbacks{
			OnCancelRun:  func() { cancels++ },
			OnBackground: func() { backgrounds++ },
		})
		d.Generating()

		d.cancelRunBtn.TypedKey(&fyne.KeyEvent{Name: fyne.KeyN})
		d.backgroundBtn.TypedKey(&fyne.KeyEvent{Name: fyne.KeyB})

		assert.Equal(t, 1, cancels)
		assert.Equal(t, 1, backgrounds)
	})

	t.Run("the letters mean nothing while no run goes", func(t *testing.T) {
		var cancels, backgrounds, closes int
		d := newTestTagsDialog(t, TagsDialogCallbacks{
			OnCancelRun:  func() { cancels++ },
			OnBackground: func() { backgrounds++ },
			OnClose:      func() { closes++ },
		})

		d.closeBtn.TypedKey(&fyne.KeyEvent{Name: fyne.KeyN})
		d.closeBtn.TypedKey(&fyne.KeyEvent{Name: fyne.KeyB})

		assert.Equal(t, 0, cancels)
		assert.Equal(t, 0, backgrounds)
		assert.Equal(t, 0, closes)
	})

	t.Run("a letter typed into an input is text and nothing else", func(t *testing.T) {
		var cancels, backgrounds int
		d := newTestTagsDialog(t, TagsDialogCallbacks{
			OnCancelRun:  func() { cancels++ },
			OnBackground: func() { backgrounds++ },
		})
		d.Generating()
		d.concept.SetText("castle")

		d.concept.TypedKey(&fyne.KeyEvent{Name: fyne.KeyN})
		d.concept.TypedKey(&fyne.KeyEvent{Name: fyne.KeyB})

		assert.Equal(t, 0, cancels)
		assert.Equal(t, 0, backgrounds)
	})
}

func TestTagsDialog_Focus(t *testing.T) {
	focused := func(d *TagsDialog) fyne.Focusable {
		return d.window.Canvas().Focused()
	}

	t.Run("the dialog opens with the caret in the concept", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.Show()

		assert.Same(t, d.concept, focused(d))
	})

	t.Run("a run moves the focus onto cancel", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.Show()

		d.Generating()

		assert.Same(t, d.cancelRunBtn, focused(d))
	})

	t.Run("a hidden dialog focuses nothing", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.Generating()

		assert.Nil(t, focused(d))
	})

	t.Run("a run landing while the user types leaves the caret alone", func(t *testing.T) {
		for name, finish := range map[string]func(*TagsDialog){
			"tags":    func(d *TagsDialog) { d.SetTags(model.Tags{Title: "A calm morning.", Keywords: fullKeywords()}) },
			"failure": func(d *TagsDialog) { d.Fail(errors.New("running claude: exit status 1")) },
		} {
			t.Run(name, func(t *testing.T) {
				d := newTestTagsDialog(t, TagsDialogCallbacks{})
				d.Show()
				d.Generating()
				d.window.Canvas().Focus(d.location)

				finish(d)

				assert.Same(t, d.location, focused(d))
			})
		}
	})

	t.Run("a finished run passes the focus to what it left behind", func(t *testing.T) {
		for name, expected := range map[string]func(*TagsDialog) (func(), fyne.Focusable){
			"tags": func(d *TagsDialog) (func(), fyne.Focusable) {
				return func() { d.SetTags(model.Tags{Title: "A calm morning.", Keywords: fullKeywords()}) }, d.title
			},
			"failure": func(d *TagsDialog) (func(), fyne.Focusable) {
				return func() { d.Fail(errors.New("running claude: exit status 1")) }, d.generateBtn
			},
			"missing binary": func(d *TagsDialog) (func(), fyne.Focusable) {
				return func() { d.Fail(fmt.Errorf("looking for claude: %w", claudebin.ErrNotFound)) }, d.pathEntry
			},
		} {
			t.Run(name, func(t *testing.T) {
				d := newTestTagsDialog(t, TagsDialogCallbacks{})
				d.Show()
				d.Generating()

				finish, want := expected(d)
				finish()

				assert.Same(t, want, focused(d))
			})
		}
	})
}

func TestTagsDialog_ShortcutRune(t *testing.T) {
	t.Run("the letter that opened the dialog stays out of the concept", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.Show()
		d.concept.TypedRune('t')

		assert.Empty(t, d.concept.Text)
	})

	t.Run("what the user types afterwards is text", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.Show()
		d.concept.TypedRune('t')

		d.concept.TypedKey(&fyne.KeyEvent{Name: fyne.KeyT})
		d.concept.TypedRune('t')

		assert.Equal(t, "t", d.concept.Text)
	})
}

func TestTagsDialog_Submit(t *testing.T) {
	t.Run("Enter in a single-line input starts a run", func(t *testing.T) {
		for name, entry := range map[string]func(*TagsDialog) *dialogEntry{
			"concept":     func(d *TagsDialog) *dialogEntry { return d.concept },
			"location":    func(d *TagsDialog) *dialogEntry { return d.location },
			"claude path": func(d *TagsDialog) *dialogEntry { return d.pathEntry },
		} {
			t.Run(name, func(t *testing.T) {
				calls := 0
				d := newTestTagsDialog(t, TagsDialogCallbacks{OnGenerate: func() { calls++ }})

				entry(d).OnSubmitted("")

				assert.Equal(t, 1, calls)
				assert.True(t, d.generateBtn.Disabled())
				assert.True(t, d.progress.Visible())
			})
		}
	})

	t.Run("a run already going is not started again", func(t *testing.T) {
		calls := 0
		d := newTestTagsDialog(t, TagsDialogCallbacks{OnGenerate: func() { calls++ }})

		d.concept.OnSubmitted("")
		d.location.OnSubmitted("")

		assert.Equal(t, 1, calls)
	})

	t.Run("Enter in the result fields stays a newline", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		for _, entry := range []*dialogEntry{d.title, d.keywords} {
			assert.True(t, entry.MultiLine)
			assert.Nil(t, entry.OnSubmitted)
		}
	})
}

func TestTagsDialog_PhysicalKeys(t *testing.T) {
	scancodes := map[glfw.Key]int{glfw.KeyN: 45, glfw.KeyB: 11}
	keys := KeyMatcher{scancode: func(key glfw.Key) int { return scancodes[key] }}

	t.Run("a run is commanded by the place of the key, not by the letter on it", func(t *testing.T) {
		for name, expected := range map[string]struct {
			scancode             int
			cancels, backgrounds int
		}{
			"cancel":     {scancode: scancodes[glfw.KeyN], cancels: 1},
			"background": {scancode: scancodes[glfw.KeyB], backgrounds: 1},
		} {
			t.Run(name, func(t *testing.T) {
				var cancels, backgrounds int
				d := newTestTagsDialogWith(t,
					TagsDialogOptions{Filename: "DSC001.JPG", Keys: keys},
					TagsDialogCallbacks{
						OnCancelRun:  func() { cancels++ },
						OnBackground: func() { backgrounds++ },
					})
				d.Generating()

				// The name is the letter a Russian layout prints on the key, which
				// is neither of the two the dialog answers to.
				d.cancelRunBtn.TypedKey(&fyne.KeyEvent{
					Name:     fyne.KeyName("Cyrillic"),
					Physical: fyne.HardwareKey{ScanCode: expected.scancode},
				})

				assert.Equal(t, expected.cancels, cancels)
				assert.Equal(t, expected.backgrounds, backgrounds)
			})
		}
	})

	t.Run("a key of another place is left alone", func(t *testing.T) {
		var cancels, backgrounds int
		d := newTestTagsDialogWith(t,
			TagsDialogOptions{Filename: "DSC001.JPG", Keys: keys},
			TagsDialogCallbacks{
				OnCancelRun:  func() { cancels++ },
				OnBackground: func() { backgrounds++ },
			})
		d.Generating()

		d.cancelRunBtn.TypedKey(&fyne.KeyEvent{Name: fyne.KeyN, Physical: fyne.HardwareKey{ScanCode: 8}})

		assert.Equal(t, 0, cancels)
		assert.Equal(t, 0, backgrounds)
	})
}
