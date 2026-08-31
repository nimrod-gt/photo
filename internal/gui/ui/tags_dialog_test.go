package ui

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
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
		assert.Equal(t, regenerateLabel, d.generateBtn.Text)
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

	t.Run("offers no saving when the settings keep the button away", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		assert.Nil(t, d.saveBtn)
	})

	// What a save writes is decided outside the dialog, so the button stays open
	// to tags no stock site would take: the sidecar keeps them either way.
	t.Run("saves whatever the fields hold", func(t *testing.T) {
		saveCalls := 0
		d := newTestTagsDialogWith(t,
			TagsDialogOptions{Filename: "DSC001.JPG", ShowSave: true},
			TagsDialogCallbacks{OnSave: func() { saveCalls++ }})
		require.NotNil(t, d.saveBtn)
		require.False(t, d.saveBtn.Disabled())

		d.keywords.SetText("lake, forest")
		test.Tap(d.saveBtn)

		assert.Equal(t, 1, saveCalls)
		assert.False(t, d.saveBtn.Disabled())
	})

	t.Run("generate disables the button and runs the callback", func(t *testing.T) {
		calls := 0
		d := newTestTagsDialog(t, TagsDialogCallbacks{OnGenerate: func() { calls++ }})

		test.Tap(d.generateBtn)

		assert.Equal(t, 1, calls)
		assert.True(t, d.generateBtn.Disabled())
		assert.True(t, d.progress.Visible())
	})

	t.Run("a run takes Save out of the row and leaves the rest in place", func(t *testing.T) {
		d := newTestTagsDialogWith(t,
			TagsDialogOptions{Filename: "DSC001.JPG", ShowSave: true}, TagsDialogCallbacks{})
		assert.Equal(t,
			[]fyne.CanvasObject{d.closeBtn, d.generateBtn, d.backgroundBtn, d.saveBtn},
			d.buttons.Objects)

		test.Tap(d.generateBtn)

		assert.Equal(t,
			[]fyne.CanvasObject{d.stopBtn, d.generateBtn, d.backgroundBtn},
			d.buttons.Objects)
	})

	t.Run("each copy button says what it copies", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		assert.Equal(t, "Copy title", d.copyTitleBtn.AccessibilityLabel())
		assert.Equal(t, "Copy keywords", d.copyKeywordsBtn.AccessibilityLabel())
	})

	t.Run("copying sits beside the field it copies", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		for i, want := range []fyne.CanvasObject{d.copyTitleBtn, d.copyKeywordsBtn} {
			row, ok := d.resultBox.Objects[i].(*fyne.Container)
			require.True(t, ok)
			assert.Same(t, []fyne.CanvasObject{d.title, d.keywords}[i], row.Objects[0])
			assert.Same(t, want, row.Objects[1].(*fyne.Container).Objects[0])
			assert.NotContains(t, d.buttons.Objects, want)
		}
	})

	t.Run("Cancel and Stop are not walked to", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		assert.NotImplements(t, (*fyne.Focusable)(nil), d.closeBtn)
		assert.NotImplements(t, (*fyne.Focusable)(nil), d.stopBtn)
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
					[]fyne.CanvasObject{d.closeBtn, d.generateBtn, d.backgroundBtn},
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
			[]fyne.CanvasObject{d.stopBtn, d.generateBtn, d.backgroundBtn},
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

	t.Run("stop and background report to the app", func(t *testing.T) {
		var stops, backgrounds, closes int
		d := newTestTagsDialog(t, TagsDialogCallbacks{
			OnStopRun:    func() { stops++ },
			OnBackground: func() { backgrounds++ },
			OnClose:      func() { closes++ },
		})
		test.Tap(d.generateBtn)

		test.Tap(d.stopBtn)
		test.Tap(d.backgroundBtn)

		assert.Equal(t, 1, stops)
		assert.Equal(t, 1, backgrounds)
		assert.Equal(t, 0, closes, "the app decides what closing means for a run")
	})

	t.Run("Background over an idle dialog starts a run and lets it go at once", func(t *testing.T) {
		var generates, backgrounds int
		d := newTestTagsDialog(t, TagsDialogCallbacks{
			OnGenerate:   func() { generates++ },
			OnBackground: func() { backgrounds++ },
		})

		test.Tap(d.backgroundBtn)

		assert.Equal(t, 1, generates)
		assert.Equal(t, 1, backgrounds)
	})

	t.Run("a stopped run leaves the fields and the Generate button as they were", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.concept.SetText("a quiet street")
		test.Tap(d.generateBtn)

		d.StopGenerating()

		assert.False(t, d.IsGenerating())
		assert.Equal(t, generateLabel, d.generateBtn.Text)
		assert.False(t, d.generateBtn.Disabled())
		assert.False(t, d.progress.Visible())
		assert.Empty(t, d.status.Text)
		assert.Equal(t, "a quiet street", d.concept.Text)
		assert.Equal(t,
			[]fyne.CanvasObject{d.closeBtn, d.generateBtn, d.backgroundBtn},
			d.buttons.Objects)
	})

	t.Run("a stopped regeneration gets the status of the tags it kept back", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.SetTags(model.Tags{Title: "A calm morning.", Keywords: fullKeywords()})
		status := d.status.Text
		require.NotEmpty(t, status)
		test.Tap(d.generateBtn)

		d.StopGenerating()

		assert.Equal(t, status, d.status.Text)
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

	t.Run("editorial reveals the date", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		assert.False(t, d.dateRow.Visible())

		d.editorial.SetChecked(true)

		assert.True(t, d.dateRow.Visible())
		require.NotNil(t, d.date.Date)
		assert.Equal(t, startOfDay(defaultTestDate), *d.date.Date, "the entry holds the day, not the moment")
	})

	t.Run("reports the ticked mark on the day of the entry", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.editorial.SetChecked(true)

		want := model.Editorial{Marked: true, Date: time.Date(2026, time.August, 18, 0, 0, 0, 0, time.UTC)}
		assert.Equal(t, want, d.Editorial(), "the time of day of the shooting date is cut off")
		assert.Equal(t, want, d.Tags().Editorial)
	})

	t.Run("an unticked box reports nothing whatever the entry holds", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		day := time.Date(2024, time.June, 13, 0, 0, 0, 0, time.UTC)

		d.date.SetDate(&day)

		assert.Equal(t, model.Editorial{}, d.Editorial())
		assert.Equal(t, model.Editorial{}, d.Tags().Editorial)
	})

	t.Run("a ticked box with the date cleared is a mark without a day", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.editorial.SetChecked(true)

		d.date.SetDate(nil)

		assert.Equal(t, model.Editorial{Marked: true}, d.Editorial())
	})

	t.Run("unchecking editorial hides the date and drops the mark", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.editorial.SetChecked(true)

		d.editorial.SetChecked(false)

		assert.False(t, d.dateRow.Visible())
		assert.False(t, d.Editorial().Marked)
	})

	t.Run("shooting date replaces the default date", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.SetPhotoInfo(model.Tags{}, time.Date(2024, time.June, 13, 10, 0, 0, 0, time.UTC))

		d.editorial.SetChecked(true)
		want := model.Editorial{Marked: true, Date: time.Date(2024, time.June, 13, 0, 0, 0, 0, time.UTC)}
		assert.Equal(t, want, d.Editorial())
		assert.False(t, d.resultBox.Visible())
	})

	t.Run("the mark the file carries ticks the box on its own day", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		marked := model.Editorial{Marked: true, Date: time.Date(2024, time.June, 13, 0, 0, 0, 0, time.UTC)}

		d.SetPhotoInfo(model.Tags{Title: "A tram climbs the hill.", Editorial: marked},
			time.Date(2024, time.July, 1, 10, 0, 0, 0, time.UTC))

		assert.True(t, d.editorial.Checked)
		assert.True(t, d.dateRow.Visible())
		assert.Equal(t, marked, d.Editorial(), "the marked day wins over the shooting date")
	})

	t.Run("a file with a mark and no tags of its own still ticks the box", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		marked := model.Editorial{Marked: true, Date: time.Date(2024, time.June, 13, 0, 0, 0, 0, time.UTC)}

		d.SetPhotoInfo(model.Tags{Editorial: marked}, time.Time{})

		assert.True(t, d.editorial.Checked)
		assert.False(t, d.resultBox.Visible())
		assert.Equal(t, marked, d.Editorial())
	})

	t.Run("a box the user ticked survives a file that marks nothing", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.editorial.SetChecked(true)

		d.SetPhotoInfo(model.Tags{}, defaultTestDate)

		assert.True(t, d.editorial.Checked)
	})

	t.Run("a box the user cleared is not ticked again by a read", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.editorial.SetChecked(true)
		d.editorial.SetChecked(false)

		d.SetPhotoInfo(model.Tags{Editorial: model.Editorial{Marked: true}}, time.Time{})

		assert.False(t, d.editorial.Checked)
		assert.False(t, d.dateRow.Visible())
	})

	// The tick the file made is not the user's answer, so clearing it is - and
	// the read that lands behind it must not put it back.
	t.Run("a box the file ticked and the user cleared stays cleared", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		marked := model.Tags{Editorial: model.Editorial{Marked: true}}
		d.SetPhotoInfo(marked, time.Time{})

		d.editorial.SetChecked(false)
		d.SetPhotoInfo(marked, time.Time{})

		assert.False(t, d.editorial.Checked)
	})

	// Two reads land for one photo - the cache first, the file behind it - and
	// the second must not undo what the first put on screen either.
	t.Run("a second read leaves the box the first one ticked alone", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		marked := model.Editorial{Marked: true, Date: time.Date(2024, time.June, 13, 0, 0, 0, 0, time.UTC)}
		d.SetPhotoInfo(model.Tags{Editorial: marked}, time.Time{})

		d.SetPhotoInfo(model.Tags{}, time.Time{})

		assert.True(t, d.editorial.Checked)
		assert.Equal(t, marked, d.Editorial())
	})

	// A cleared day is an answer of its own, and the day the shutter went is not
	// it: filling the entry would hand the mark a day nobody picked and write it
	// out on the very next close.
	t.Run("a mark the file names no day for leaves the entry empty", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		marked := model.Tags{Editorial: model.Editorial{Marked: true}}

		d.SetPhotoInfo(marked, defaultTestDate)

		assert.True(t, d.editorial.Checked)
		assert.Nil(t, d.date.Date, "the day of the shot is not the day of the mark")
		assert.Equal(t, model.Editorial{Marked: true}, d.Editorial())
		assert.True(t, marked.Equal(d.Tags()), "an opened dialog has nothing to write back")
	})

	// The handed day is the same one the entry was seeded with, so nothing about
	// the entry says it was answered for; the dialog has to remember that it was.
	t.Run("the handed day outlives the read landing behind it", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		handed := model.Editorial{Marked: true, Date: startOfDay(defaultTestDate)}

		d.RestoreTags(model.Tags{Title: "A tram climbs the hill.", Editorial: handed})
		d.SetPhotoInfo(model.Tags{Editorial: model.Editorial{Marked: true,
			Date: time.Date(2020, time.January, 2, 0, 0, 0, 0, time.UTC)}}, defaultTestDate)

		assert.Equal(t, handed, d.Editorial(), "the file is the older answer")
	})

	// A shown entry parses its own text back whenever that text changes, and text
	// names no zone, so what comes back is UTC. A day seeded in any other zone
	// would never read back equal to the default it was stored as, and every
	// later seed would take the entry for one the user picked by hand.
	t.Run("a shooting moment in another zone seeds the day it names there", func(t *testing.T) {
		d := newTestTagsDialogWith(t, TagsDialogOptions{Filename: "DSC001.JPG"}, TagsDialogCallbacks{})
		west := time.FixedZone("west", -7*60*60)
		taken := time.Date(2026, time.August, 18, 23, 30, 0, 0, west)
		// The entry reparses only once it is shown, which is where the dialog
		// always is by the time a read lands on it.
		test.WidgetRenderer(d.date)

		d.SetPhotoInfo(model.Tags{}, taken)
		d.editorial.SetChecked(true)

		assert.True(t, d.dateUntouched(), "a seeded day is nobody's pick")
		assert.Equal(t, model.Editorial{Marked: true,
			Date: time.Date(2026, time.August, 18, 0, 0, 0, 0, time.UTC)}, d.Editorial())
	})

	t.Run("restoring an unmarked value clears the box the file ticked", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.SetPhotoInfo(model.Tags{Editorial: model.Editorial{Marked: true}}, defaultTestDate)

		d.RestoreTags(model.Tags{Title: "A tram climbs the hill."})

		assert.False(t, d.editorial.Checked)
		assert.False(t, d.dateRow.Visible())
		assert.Equal(t, model.Editorial{}, d.Editorial())
	})

	t.Run("restoring a marked value fills the entry with its day", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		marked := model.Editorial{Marked: true, Date: time.Date(2024, time.June, 13, 0, 0, 0, 0, time.UTC)}

		d.RestoreTags(model.Tags{Title: "A tram climbs the hill.", Editorial: marked})

		assert.True(t, d.editorial.Checked)
		assert.True(t, d.dateRow.Visible())
		assert.Equal(t, marked, d.Editorial())
	})

	// The dialog that closed had the box ticked and its entry empty, so the day
	// this one was seeded with is not the user's answer and must not come back.
	t.Run("restoring a mark without a day empties the entry", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.RestoreTags(model.Tags{Title: "A tram climbs the hill.", Editorial: model.Editorial{Marked: true}})

		assert.True(t, d.editorial.Checked)
		assert.Nil(t, d.date.Date)
		assert.Equal(t, model.Editorial{Marked: true}, d.Editorial())
	})

	t.Run("a restored box is not ticked again by the file behind it", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.RestoreTags(model.Tags{Title: "A tram climbs the hill."})

		d.SetPhotoInfo(model.Tags{Editorial: model.Editorial{Marked: true}}, time.Time{})

		assert.False(t, d.editorial.Checked)
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
			TagsDialogOptions{Filename: "DSC001.JPG", ShowSave: true}, TagsDialogCallbacks{})

		d.SetPhotoInfo(model.Tags{Title: "Old title.", Keywords: []string{"lake", "forest"}}, time.Time{})

		assert.True(t, d.resultBox.Visible())
		assert.Equal(t, "Old title.", d.title.Text)
		assert.Equal(t, "lake, forest", d.keywords.Text)
		assert.Equal(t, model.Tags{Title: "Old title.", Keywords: []string{"lake", "forest"}}, d.Tags())
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

	t.Run("the typed concept is what the tags carry", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.concept.SetText("  tram 28 seen head-on  ")

		assert.Equal(t, "tram 28 seen head-on", d.Tags().Concept)
	})

	t.Run("a concept read from the file fills the empty entry", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.SetPhotoInfo(model.Tags{
			Title:    "Old title.",
			Keywords: []string{"lake"},
			Concept:  "tram 28 seen head-on",
		}, time.Time{})

		assert.Equal(t, "tram 28 seen head-on", d.concept.Text)
		assert.Equal(t, "tram 28 seen head-on", d.Tags().Concept)
	})

	// The note is what the tags were generated from, so it is worth having back
	// even on a photo whose tags were never saved beside it.
	t.Run("a concept read from a file without tags fills the entry", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})

		d.SetPhotoInfo(model.Tags{Concept: "tram 28 seen head-on"}, time.Time{})

		assert.Equal(t, "tram 28 seen head-on", d.concept.Text)
		assert.True(t, d.resultBox.Hidden, "there are no tags to show")
	})

	t.Run("a concept read from the file leaves a typed one alone", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.concept.SetText("fog over the lake")

		d.SetPhotoInfo(model.Tags{
			Title:    "Old title.",
			Keywords: []string{"lake"},
			Concept:  "tram 28 seen head-on",
		}, time.Time{})

		assert.Equal(t, "fog over the lake", d.concept.Text)
	})

	t.Run("a concept typed during the run survives the tags it lands with", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.Generating()
		d.concept.SetText("fog over the lake")

		d.SetTags(model.Tags{
			Title:    "A calm morning.",
			Keywords: fullKeywords(),
			Concept:  "tram 28 seen head-on",
		})

		assert.Equal(t, "fog over the lake", d.concept.Text)
		assert.Equal(t, "fog over the lake", d.Tags().Concept)
	})

	// A run only ever echoes back the note it was asked with, so a note deleted
	// while it went is the user's answer and the landing run leaves it deleted.
	t.Run("a concept cleared during the run is not filled back in", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.concept.SetText("tram 28 seen head-on")
		d.Generating()
		d.concept.SetText("")

		d.SetTags(model.Tags{
			Title:    "A calm morning.",
			Keywords: fullKeywords(),
			Concept:  "tram 28 seen head-on",
		})

		assert.Empty(t, d.concept.Text)
		assert.Empty(t, d.Tags().Concept)
	})

	t.Run("a location cleared during the run is not filled back in", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.location.SetText("Lisbon")
		d.Generating()
		d.location.SetText("")

		d.SetTags(model.Tags{
			Title:    "A calm morning.",
			Keywords: fullKeywords(),
			Place:    model.Place{Location: "Lisbon", City: "Lisbon", Country: "Portugal"},
		})

		assert.Empty(t, d.location.Text)
		assert.Empty(t, d.Tags().Place.Location)
	})

	// The dialog that closed handed over everything it held, so what comes back
	// goes in whole - over the seed this one read out of the cache, which is the
	// older of the two.
	t.Run("restored fields replace the ones seeded from the file", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.SetPhotoInfo(model.Tags{
			Title:    "The old title.",
			Keywords: fullKeywords(),
			Place:    model.Place{Location: "Porto", City: "Porto"},
			Concept:  "the old note",
		}, time.Time{})
		d.Generating()

		d.RestoreTags(model.Tags{
			Title:    "The handed title.",
			Keywords: []string{"tram"},
			Place:    model.Place{Location: "Lisbon"},
			Concept:  "the handed note",
		})

		assert.Equal(t, "Lisbon", d.location.Text)
		assert.Equal(t, "the handed note", d.concept.Text)
		assert.Equal(t, "The handed title.", d.title.Text)
	})

	t.Run("a restored dialog that handed over an empty field leaves it empty", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.SetPhotoInfo(model.Tags{Concept: "the old note"}, time.Time{})
		d.Generating()

		d.RestoreTags(model.Tags{Title: "A calm morning.", Keywords: fullKeywords()})

		assert.Empty(t, d.concept.Text)
	})
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
				generates := 0
				d := newTestTagsDialog(t, TagsDialogCallbacks{OnGenerate: func() { generates++ }})

				d.generateBtn.TypedKey(&fyne.KeyEvent{Name: key})

				assert.Equal(t, 1, generates)
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

	t.Run("a run takes the focus off the button it disables", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.Show()
		d.window.Canvas().Focus(d.generateBtn)

		d.Generating()

		assert.Same(t, d.backgroundBtn, focused(d))
	})

	t.Run("a run takes the focus off a button that left the row", func(t *testing.T) {
		d := newTestTagsDialogWith(t,
			TagsDialogOptions{Filename: "DSC001.JPG", ShowSave: true}, TagsDialogCallbacks{})
		d.Show()
		d.SetTags(model.Tags{Title: "A calm morning.", Keywords: fullKeywords()})
		d.window.Canvas().Focus(d.saveBtn)

		d.Generating()

		assert.NotContains(t, d.buttons.Objects, fyne.CanvasObject(d.saveBtn))
		assert.Same(t, d.backgroundBtn, focused(d), "Space must not press a button that is off screen")
	})

	t.Run("a run started with the mouse gives the dialog the keyboard back", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.Show()
		// A tap drops the canvas focus on its way past the button it presses.
		d.window.Canvas().Unfocus()

		d.Generating()

		assert.Same(t, d.backgroundBtn, focused(d))
	})

	t.Run("a run leaves a caret in a field where it is", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.Show()

		d.Generating()

		assert.Same(t, d.concept, focused(d))
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
				d.window.Canvas().Focus(d.backgroundBtn)

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

	// A character behind a modifier - AltGr+8 for [ on a German keyboard,
	// Option+8 for the bullet on a Mac - never arrives as a key event: the driver
	// turns the chord into a shortcut and delivers the rune alone afterwards.
	t.Run("a character typed with a modifier is text", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.Show()

		d.concept.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.Key8, Modifier: fyne.KeyModifierAlt})
		d.concept.TypedRune('[')

		assert.Equal(t, "[", d.concept.Text)
	})
}

func TestTagsDialog_Chords(t *testing.T) {
	shift := &fyne.KeyEvent{Name: desktop.KeyShiftLeft}
	enter := &fyne.KeyEvent{Name: fyne.KeyReturn}
	chord := func(modifier fyne.KeyModifier, key fyne.KeyName) *desktop.CustomShortcut {
		return &desktop.CustomShortcut{KeyName: key, Modifier: modifier}
	}

	t.Run("Shift+Enter starts a run wherever the focus sits", func(t *testing.T) {
		for name, press := range map[string]func(*TagsDialog){
			"concept":   func(d *TagsDialog) { d.concept.KeyDown(shift); d.concept.TypedKey(enter) },
			"keywords":  func(d *TagsDialog) { d.keywords.KeyDown(shift); d.keywords.TypedKey(enter) },
			"editorial": func(d *TagsDialog) { d.editorial.KeyDown(shift); d.editorial.TypedKey(enter) },
			// The Shift and the Enter need not be heard by the same widget: the
			// button takes the modifier, and the field the key that follows it.
			"a button": func(d *TagsDialog) { d.generateBtn.KeyDown(shift); d.concept.TypedKey(enter) },
		} {
			t.Run(name, func(t *testing.T) {
				calls := 0
				d := newTestTagsDialog(t, TagsDialogCallbacks{OnGenerate: func() { calls++ }})
				d.Show()

				press(d)

				assert.Equal(t, 1, calls)
				assert.True(t, d.generateBtn.Disabled())
				assert.True(t, d.progress.Visible())
			})
		}
	})

	t.Run("a bare Enter starts nothing", func(t *testing.T) {
		calls := 0
		d := newTestTagsDialog(t, TagsDialogCallbacks{OnGenerate: func() { calls++ }})
		d.Show()

		d.concept.TypedKey(enter)

		assert.Equal(t, 0, calls)
	})

	t.Run("a run already going is not started again", func(t *testing.T) {
		calls := 0
		d := newTestTagsDialog(t, TagsDialogCallbacks{OnGenerate: func() { calls++ }})

		d.Show()

		d.concept.KeyDown(shift)
		d.concept.TypedKey(enter)
		d.concept.KeyDown(shift)
		d.concept.TypedKey(enter)

		assert.Equal(t, 1, calls)
	})

	t.Run("the Shift is forgotten when it goes up", func(t *testing.T) {
		calls := 0
		d := newTestTagsDialog(t, TagsDialogCallbacks{OnGenerate: func() { calls++ }})

		d.Show()

		d.concept.KeyDown(shift)
		d.concept.KeyUp(shift)
		d.concept.TypedKey(enter)

		assert.Equal(t, 0, calls)
	})

	t.Run("the Shift released over another widget is forgotten", func(t *testing.T) {
		calls := 0
		d := newTestTagsDialog(t, TagsDialogCallbacks{OnGenerate: func() { calls++ }})

		d.Show()

		d.concept.KeyDown(shift)
		d.location.KeyUp(shift)
		d.location.TypedKey(enter)

		assert.Equal(t, 0, calls)
	})

	// Shift+Tab is answered by the driver itself, so the field the focus leaves
	// never hears the Tab and the Shift is still down where the focus lands.
	t.Run("a Shift held across a walk backwards still generates", func(t *testing.T) {
		calls := 0
		d := newTestTagsDialog(t, TagsDialogCallbacks{OnGenerate: func() { calls++ }})

		d.Show()

		d.location.KeyDown(shift)
		d.location.FocusLost()
		d.concept.FocusGained()
		d.concept.TypedKey(enter)

		assert.Equal(t, 1, calls)
	})

	t.Run("Ctrl+Enter lets a running generation go", func(t *testing.T) {
		for name, modifier := range map[string]fyne.KeyModifier{
			"control": fyne.KeyModifierControl,
			"command": fyne.KeyModifierSuper,
		} {
			t.Run(name, func(t *testing.T) {
				var generates, backgrounds int
				d := newTestTagsDialog(t, TagsDialogCallbacks{
					OnGenerate:   func() { generates++ },
					OnBackground: func() { backgrounds++ },
				})
				d.Generating()

				d.concept.TypedShortcut(chord(modifier, fyne.KeyReturn))

				assert.Equal(t, 0, generates, "the run that is going is the one let go of")
				assert.Equal(t, 1, backgrounds)
			})
		}
	})

	t.Run("Ctrl+Enter over an idle dialog starts a run and lets it go at once", func(t *testing.T) {
		for name, widget := range map[string]func(*TagsDialog, fyne.Shortcut){
			"an input": func(d *TagsDialog, s fyne.Shortcut) { d.concept.TypedShortcut(s) },
			"a check":  func(d *TagsDialog, s fyne.Shortcut) { d.editorial.TypedShortcut(s) },
			"a button": func(d *TagsDialog, s fyne.Shortcut) { d.generateBtn.TypedShortcut(s) },
			"the date": func(d *TagsDialog, s fyne.Shortcut) { d.date.TypedShortcut(s) },
		} {
			t.Run(name, func(t *testing.T) {
				var generates, backgrounds int
				d := newTestTagsDialog(t, TagsDialogCallbacks{
					OnGenerate:   func() { generates++ },
					OnBackground: func() { backgrounds++ },
				})

				widget(d, chord(fyne.KeyModifierControl, fyne.KeyReturn))

				assert.Equal(t, 1, generates)
				assert.Equal(t, 1, backgrounds)
			})
		}
	})

	t.Run("a chord of another key is left to the input", func(t *testing.T) {
		var generates, backgrounds int
		d := newTestTagsDialog(t, TagsDialogCallbacks{
			OnGenerate:   func() { generates++ },
			OnBackground: func() { backgrounds++ },
		})
		d.concept.SetText("castle")

		d.concept.TypedShortcut(&fyne.ShortcutSelectAll{})

		assert.Equal(t, "castle", d.concept.SelectedText())
		assert.Equal(t, 0, generates)
		assert.Equal(t, 0, backgrounds)
	})
}

func TestTagsDialog_TagChords(t *testing.T) {
	chord := func(key fyne.KeyName) *desktop.CustomShortcut {
		return &desktop.CustomShortcut{KeyName: key, Modifier: fyne.KeyModifierAlt}
	}

	t.Run("Alt+C and Alt+V reach the app wherever the focus sits", func(t *testing.T) {
		for name, press := range map[string]func(*TagsDialog, fyne.Shortcut){
			"an input": func(d *TagsDialog, s fyne.Shortcut) { d.concept.TypedShortcut(s) },
			"a check":  func(d *TagsDialog, s fyne.Shortcut) { d.editorial.TypedShortcut(s) },
			"a button": func(d *TagsDialog, s fyne.Shortcut) { d.generateBtn.TypedShortcut(s) },
			"the date": func(d *TagsDialog, s fyne.Shortcut) { d.date.TypedShortcut(s) },
		} {
			t.Run(name, func(t *testing.T) {
				var copies, pastes int
				d := newTestTagsDialog(t, TagsDialogCallbacks{
					OnCopyTags:  func() { copies++ },
					OnPasteTags: func() { pastes++ },
				})

				press(d, chord(fyne.KeyC))
				press(d, chord(fyne.KeyV))

				assert.Equal(t, 1, copies)
				assert.Equal(t, 1, pastes)
			})
		}
	})

	// Option+C is how a Mac keyboard types ç: the driver reports the chord and
	// the character it composed both, and the character must not land in the
	// field the chord was pressed over.
	t.Run("the character the chord composes stays out of the field", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{OnCopyTags: func() {}})
		d.Show()
		d.concept.TypedKey(&fyne.KeyEvent{Name: fyne.KeyT})

		d.concept.TypedShortcut(chord(fyne.KeyC))
		d.concept.TypedRune('ç')

		assert.Empty(t, d.concept.Text)
	})

	// Windows and Linux compose nothing from Alt+C, so nothing is left over
	// there; the letter typed next announces itself as a key event first, which
	// is what tells the field the rune behind it is a real one.
	t.Run("the letter typed after the chord is text", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{OnCopyTags: func() {}})
		d.Show()

		d.concept.TypedShortcut(chord(fyne.KeyC))
		d.concept.TypedKey(&fyne.KeyEvent{Name: fyne.KeyA})
		d.concept.TypedRune('a')

		assert.Equal(t, "a", d.concept.Text)
	})

	t.Run("a chord nothing answers is left to the input", func(t *testing.T) {
		var copies, pastes int
		d := newTestTagsDialog(t, TagsDialogCallbacks{
			OnCopyTags:  func() { copies++ },
			OnPasteTags: func() { pastes++ },
		})

		d.concept.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyC, Modifier: fyne.KeyModifierControl})

		assert.Equal(t, 0, copies, "Ctrl+C belongs to the text in the field")
		assert.Equal(t, 0, pastes)
	})

	// Fyne names the key of a chord after the letter the layout prints on it and
	// gives up when that letter is not ASCII, so a Cyrillic layout sends the
	// chord along with no name; the place of the key is what it is answered by.
	t.Run("a chord typed in another layout goes by the place of the key", func(t *testing.T) {
		stubScanCodes(t)
		var copies, pastes int
		d := newTestTagsDialog(t, TagsDialogCallbacks{
			OnCopyTags:  func() { copies++ },
			OnPasteTags: func() { pastes++ },
		})

		pressPlace(d, scanCodeC)
		pressPlace(d, scanCodeV)

		assert.Equal(t, 1, copies)
		assert.Equal(t, 1, pastes)
	})

	t.Run("a nameless chord over another key is left to the input", func(t *testing.T) {
		stubScanCodes(t)
		var copies, pastes int
		d := newTestTagsDialog(t, TagsDialogCallbacks{
			OnCopyTags:  func() { copies++ },
			OnPasteTags: func() { pastes++ },
		})

		pressPlace(d, scanCodeC+scanCodeV)

		assert.Equal(t, 0, copies)
		assert.Equal(t, 0, pastes)
	})

	// A driver that reports no scan code at all would otherwise have every
	// nameless chord answered as the key whose place is unknown too.
	t.Run("a nameless chord with no key behind it does nothing", func(t *testing.T) {
		stubScanCodes(t)
		var copies, pastes int
		d := newTestTagsDialog(t, TagsDialogCallbacks{
			OnCopyTags:  func() { copies++ },
			OnPasteTags: func() { pastes++ },
		})

		d.concept.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyUnknown, Modifier: fyne.KeyModifierAlt})

		assert.Equal(t, 0, copies)
		assert.Equal(t, 0, pastes)
	})
}

const (
	scanCodeC = 8
	scanCodeV = 9
)

// GLFW answers with nothing until a window of its own is open, which no test has.
func stubScanCodes(t *testing.T) {
	t.Helper()
	previous := keyScanCode
	keyScanCode = func(key glfw.Key) int {
		switch key {
		case glfw.KeyC:
			return scanCodeC
		case glfw.KeyV:
			return scanCodeV
		}
		return 0
	}
	t.Cleanup(func() { keyScanCode = previous })
}

func pressPlace(d *TagsDialog, scanCode int) {
	d.concept.KeyDown(&fyne.KeyEvent{Name: fyne.KeyUnknown, Physical: fyne.HardwareKey{ScanCode: scanCode}})
	d.concept.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyUnknown, Modifier: fyne.KeyModifierAlt})
	d.concept.KeyUp(&fyne.KeyEvent{Name: fyne.KeyUnknown, Physical: fyne.HardwareKey{ScanCode: scanCode}})
}

func TestTagsDialog_PasteTags(t *testing.T) {
	pasted := model.Tags{Title: "A calm morning by the lake.", Keywords: fullKeywords()}

	t.Run("fills the result fields and leaves the free text alone", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		d.concept.SetText("a note of its own")
		d.location.SetText("Riga, Latvia")

		d.PasteTags(pasted)

		assert.Equal(t, pasted.Title, d.title.Text)
		assert.Equal(t, pasted.KeywordLine(), d.keywords.Text)
		assert.Equal(t, "a note of its own", d.concept.Text)
		assert.Equal(t, "Riga, Latvia", d.location.Text)
		assert.True(t, d.resultBox.Visible())
		assert.Contains(t, d.status.Text, "50 keywords")
	})

	// The Generate button is a run's to rename; a paste is not a run.
	t.Run("leaves the buttons where a run would have moved them", func(t *testing.T) {
		d := newTestTagsDialogWith(t, TagsDialogOptions{Filename: "DSC001.JPG", ShowSave: true}, TagsDialogCallbacks{})

		d.PasteTags(pasted)

		assert.Equal(t, generateLabel, d.generateBtn.Text)
		assert.Contains(t, d.buttons.Objects, fyne.CanvasObject(d.saveBtn), "a paste is no run")
	})

	t.Run("reports whether a paste would replace anything", func(t *testing.T) {
		d := newTestTagsDialog(t, TagsDialogCallbacks{})
		assert.False(t, d.HasTags())

		d.PasteTags(pasted)

		assert.True(t, d.HasTags())
	})
}
