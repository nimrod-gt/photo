package app

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"photo/internal/core/imaging"
	"photo/internal/core/model"
	"photo/internal/core/tags"
)

const testConcept = "tram 28 seen head-on"

func TestNothingToWrite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tags model.Tags
		want bool
	}{
		{name: "nothing at all", tags: model.Tags{}, want: true},
		{name: "blank title alone", tags: model.Tags{Title: "   "}, want: true},
		{name: "a title", tags: model.Tags{Title: "A tram climbs the hill."}},
		{name: "keywords", tags: model.Tags{Keywords: []string{"lisbon"}}},
		{name: "a location the user typed", tags: model.Tags{Place: model.Place{Location: "Cascais"}}},
		{name: "a split without free text", tags: model.Tags{Place: model.Place{City: "Cascais"}}},
		{name: "a blank location", tags: model.Tags{Place: model.Place{Location: "  "}}, want: true},
		{name: "a concept the user typed", tags: model.Tags{Concept: testConcept}},
		{name: "a blank concept", tags: model.Tags{Concept: "  "}, want: true},
		{name: "an editorial mark", tags: model.Tags{Editorial: editorialMark()}},
		{name: "a mark without a day", tags: model.Tags{Editorial: model.Editorial{Marked: true}}},
		{name: "a day nobody marked", tags: model.Tags{Editorial: model.Editorial{Date: editorialDay()}}, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, nothingToWrite(tc.tags))
		})
	}
}

func TestWriteNote(t *testing.T) {
	t.Parallel()

	assert.Empty(t, writeNote(imaging.StockWrite{}))
	assert.Equal(t, rewrittenNote, writeNote(imaging.StockWrite{Rewritten: true}))
	assert.Equal(t, placeDroppedNote, writeNote(imaging.StockWrite{PlaceDropped: true}))
	assert.Equal(t, conceptDroppedNote, writeNote(imaging.StockWrite{ConceptDropped: true}))
	assert.Equal(t, editorialDroppedNote, writeNote(imaging.StockWrite{EditorialDropped: true}))
	assert.Equal(t, rewrittenNote+"; "+placeDroppedNote+"; "+conceptDroppedNote+"; "+editorialDroppedNote,
		writeNote(imaging.StockWrite{
			Rewritten:        true,
			PlaceDropped:     true,
			ConceptDropped:   true,
			EditorialDropped: true,
		}))
}

// The Fyne test driver keeps global state, so this one shares the app the
// runner tests use and does not run in parallel.
func TestTagsSessionClose(t *testing.T) {
	a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))
	photo := testPhoto(t, true)
	session := a.openTestTagsDialog(t, photo)

	typeIntoDialog(session, typedTags())
	session.close()

	assert.Equal(t, typedTags(), sidecarTags(t, photo))
}

// A JPEG with no RAW pair has a sidecar of its own, named after the image.
func TestTagsSessionCloseWithoutARAWPair(t *testing.T) {
	a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))
	photo := testPhoto(t, false)
	require.False(t, photo.HasRAW())
	session := a.openTestTagsDialog(t, photo)

	typeIntoDialog(session, typedTags())
	session.close()

	assert.Equal(t, typedTags(), sidecarTags(t, photo))
	assert.FileExists(t, strings.TrimSuffix(photo.ImagePath, ".JPG")+".xmp")

	reopened := a.openTestTagsDialog(t, photo)
	reopened.seed()

	assert.Equal(t, typedTags(), reopened.dialog.Tags(),
		"the dialog opens on what the last one saved")
}

// The note is what a generation is made from, so it is worth a sidecar before
// there are any tags to go with it.
func TestTagsSessionCloseWithAConceptAlone(t *testing.T) {
	a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))
	photo := testPhoto(t, true)
	session := a.openTestTagsDialog(t, photo)

	typeIntoDialog(session, model.Tags{Concept: testConcept})
	session.close()

	require.Eventually(t, func() bool {
		written, err := imaging.ReadSidecar(photo.SidecarPath())
		return err == nil && written.Concept == testConcept
	}, time.Second, 5*time.Millisecond, "the sidecar was never written")

	reopened := a.openTestTagsDialog(t, photo)
	reopened.seed()

	assert.Equal(t, testConcept, reopened.dialog.Concept(),
		"the dialog opens on what the last one saved")
}

// The mark is saved like the concept note: on its own it is worth a sidecar,
// and the dialog that reopens over it comes back ticked on the same day.
func TestTagsSessionCloseWithAnEditorialMarkAlone(t *testing.T) {
	a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))
	photo := testPhoto(t, true)
	session := a.openTestTagsDialog(t, photo)

	typeIntoDialog(session, model.Tags{Editorial: editorialMark()})
	session.close()

	awaitSidecar(t, photo, model.Tags{Editorial: editorialMark()})

	reopened := a.openTestTagsDialog(t, photo)
	reopened.seed()

	assert.Equal(t, editorialMark(), reopened.dialog.Editorial(),
		"the dialog opens on the mark the last one saved")
}

// A mark the user left without a day keeps it: the shooting date the dialog
// seeds its entry with is not an answer, so an opened and closed dialog has
// nothing to write back and the file keeps the mark as it stood.
func TestTagsSessionCloseOverAMarkWithoutADay(t *testing.T) {
	a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))
	photo := testPhoto(t, true)
	marked := model.Tags{Editorial: model.Editorial{Marked: true}}
	a.imageProvider.StoreStockInfo(photo.ImagePath, imaging.StockInfo{Tags: marked, Taken: editorialDay()})

	session := a.openTestTagsDialog(t, photo)
	session.seed()
	session.close()

	assert.Equal(t, marked.Editorial, session.dialog.Editorial())
	require.Never(t, func() bool {
		written, err := imaging.ReadSidecar(photo.SidecarPath())
		return err == nil && !written.Editorial.Date.IsZero()
	}, 200*time.Millisecond, 20*time.Millisecond, "a day nobody picked is not written")
}

func editorialMark() model.Editorial {
	return model.Editorial{Marked: true, Date: editorialDay()}
}

func editorialDay() time.Time {
	return time.Date(2026, time.June, 13, 0, 0, 0, 0, time.UTC)
}

func existingSidecar() model.Tags {
	return model.Tags{Title: "What the file already says.", Keywords: []string{"lake", "morning"}}
}

func writeTestSidecar(t *testing.T, photo model.Photo, written model.Tags) {
	t.Helper()
	require.NoError(t, imaging.WriteSidecar(photo.SidecarPath(), written))
}

func awaitSidecar(t *testing.T, photo model.Photo, want model.Tags) {
	t.Helper()
	path := photo.SidecarPath()
	var written model.Tags
	require.Eventuallyf(t, func() bool {
		var err error
		written, err = imaging.ReadSidecar(path)
		return err == nil && written.Equal(want)
	}, time.Second, 5*time.Millisecond, "the sidecar holds %+v, expected %+v", &written, want)
}

// A dialog can be closed before the read of its photo lands, and everything on
// it was then typed into fields that stood empty for want of that read. Saving
// such a dialog over the file would take the tags nobody had seen with it.
func TestTagsSessionSaveWithoutKnowingTheFile(t *testing.T) {
	t.Run("what was typed is added to what the file holds", func(t *testing.T) {
		a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))
		photo := testPhoto(t, true)
		writeTestSidecar(t, photo, existingSidecar())
		session := a.openTestTagsDialog(t, photo)
		require.False(t, session.known, "the cache holds nothing for this photo")

		typeIntoDialog(session, model.Tags{Concept: testConcept})
		session.close()

		want := existingSidecar()
		want.Concept = testConcept
		awaitSidecar(t, photo, want)
	})

	t.Run("a dialog that knows the file still clears what was emptied", func(t *testing.T) {
		a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))
		photo := testPhoto(t, true)
		writeTestSidecar(t, photo, existingSidecar())
		a.imageProvider.StoreStockInfo(photo.ImagePath, imaging.StockInfo{Tags: existingSidecar()})
		session := a.openTestTagsDialog(t, photo)
		session.seed()
		require.True(t, session.known)

		// The fields are put back whole, which is what emptying the title and
		// the keywords by hand leaves the dialog holding.
		session.dialog.RestoreTags(model.Tags{Concept: testConcept})
		session.close()

		awaitSidecar(t, photo, model.Tags{Concept: testConcept})
	})

	t.Run("the fields handed to a failed run are added the same way", func(t *testing.T) {
		held := newHeldTagger(model.Tags{}, errors.New("claude fell over"))
		a := newTestApplication(t, held)
		photo := testPhoto(t, true)
		writeTestSidecar(t, photo, existingSidecar())
		session := a.openTestTagsDialog(t, photo)

		a.tagRuns.start(session, tags.Request{Photo: photo})
		<-held.started
		typeIntoDialog(session, model.Tags{Concept: testConcept})
		session.background()

		close(held.release)

		want := existingSidecar()
		want.Concept = testConcept
		awaitSidecar(t, photo, want)
	})

	t.Run("the fields flushed on the way out are added the same way", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, true)
		writeTestSidecar(t, photo, existingSidecar())
		session := a.openTestTagsDialog(t, photo)

		a.tagRuns.start(session, tags.Request{Photo: photo})
		<-held.started
		typeIntoDialog(session, model.Tags{Concept: testConcept})
		session.background()

		a.tagRuns.stopAll()

		want := existingSidecar()
		want.Concept = testConcept
		awaitSidecar(t, photo, want)
	})
}

func uploadableTags() model.Tags {
	keywords := make([]string, 0, model.KeywordCount)
	for i := range model.KeywordCount {
		keywords = append(keywords, fmt.Sprintf("keyword %d", i))
	}
	return model.Tags{Title: "A tram climbs the hill.", Keywords: keywords}
}

// What a save writes is settled here rather than in the dialog, because the
// automatic saves ask the same question the button does.
func TestTagsSessionWritePlan(t *testing.T) {
	a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))
	jpeg := testPhoto(t, true)
	raw := testPhotoNamed(t, "DSC002.ARW")

	tests := []struct {
		name    string
		photo   model.Photo
		written model.Tags
		saved   model.Tags
		want    tagWrite
		manual  bool
		wanted  tagWrite
	}{
		{
			name:    "the settings say nothing is written",
			photo:   jpeg,
			written: uploadableTags(),
		},
		{
			name:    "the sidecar is written automatically",
			photo:   jpeg,
			written: typedTags(),
			want:    tagWrite{sidecar: true},
			wanted:  tagWrite{sidecar: true},
		},
		{
			name:    "tags the sidecar already holds are left alone",
			photo:   jpeg,
			written: typedTags(),
			saved:   typedTags(),
			want:    tagWrite{sidecar: true},
		},
		{
			name:    "the button writes them anyway",
			photo:   jpeg,
			written: typedTags(),
			saved:   typedTags(),
			manual:  true,
			want:    tagWrite{sidecar: true},
			wanted:  tagWrite{sidecar: true},
		},
		{
			name:    "a JPEG takes tags a stock site would take",
			photo:   jpeg,
			written: uploadableTags(),
			want:    tagWrite{sidecar: true, jpeg: true},
			wanted:  tagWrite{sidecar: true, jpeg: true},
		},
		{
			name:    "tags both files already hold are left alone",
			photo:   jpeg,
			written: uploadableTags(),
			saved:   uploadableTags(),
			want:    tagWrite{sidecar: true, jpeg: true},
		},
		{
			name:    "a JPEG is left alone until the tags are ready",
			photo:   jpeg,
			written: typedTags(),
			want:    tagWrite{sidecar: true, jpeg: true},
			wanted:  tagWrite{sidecar: true},
		},
		{
			name:    "a photo that is no JPEG has nothing to write into",
			photo:   raw,
			written: uploadableTags(),
			want:    tagWrite{sidecar: true, jpeg: true},
			wanted:  tagWrite{sidecar: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &tagsSession{app: a, photo: tt.photo, saved: tt.saved}
			assert.Equal(t, tt.wanted, session.writePlan(tt.written, tt.want, tt.manual))
		})
	}
}

// With the sidecar left to the button, closing the dialog writes nothing at all
// and the tags wait on screen for the save the user asks for.
func TestTagsSessionCloseWithAutoSaveOff(t *testing.T) {
	a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))
	a.autoSaveXMP = false
	photo := testPhoto(t, true)
	session := a.openTestTagsDialog(t, photo)

	typeIntoDialog(session, typedTags())
	session.close()

	assert.Never(t, func() bool {
		_, err := os.Stat(photo.SidecarPath())
		return err == nil
	}, 200*time.Millisecond, 5*time.Millisecond, "the sidecar was written without being asked for")

	reopened := a.openTestTagsDialog(t, photo)
	typeIntoDialog(reopened, typedTags())
	reopened.save()

	assert.Equal(t, typedTags(), sidecarTags(t, photo))
}

// A run with the autosaves off leaves its tags in the cache, and the cache is
// what the next dialog takes for the contents of the sidecar. No file holds
// them, so that dialog has to write them rather than believe them.
func TestTagsCachedByARunAreNotSaved(t *testing.T) {
	held := newHeldTagger(generatedTags(), nil)
	a := newTestApplication(t, held)
	a.autoSaveXMP = false
	photo := testPhoto(t, true)
	session := a.openTestTagsDialog(t, photo)

	a.tagRuns.start(session, tags.Request{Photo: photo})
	<-held.started
	session.background()
	held.answerWithTags(t, a, photo.ImagePath)
	settleRuns(t, a, photo.ImagePath)
	require.NoFileExists(t, photo.SidecarPath())

	a.autoSaveXMP = true
	reopened := a.openTestTagsDialog(t, photo)
	reopened.seed()
	require.True(t, reopened.known, "the run filled the cache")
	reopened.close()

	awaitSidecar(t, photo, generatedTags())
}

// The Save button is asked for by hand, so it writes tags the sidecar already
// holds - but a photo with nothing to say still gets no file.
func TestTagsSessionSaveWithNothingTyped(t *testing.T) {
	a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))
	a.autoSaveXMP = false
	photo := testPhoto(t, true)
	session := a.openTestTagsDialog(t, photo)

	session.save()

	assert.Never(t, func() bool {
		_, err := os.Stat(photo.SidecarPath())
		return err == nil
	}, 200*time.Millisecond, 5*time.Millisecond, "an empty sidecar was written")
}

// The JPEG is the photo itself, so a save by hand tells the user when the tags
// were not fit for it.
func TestTagsSessionSkipNote(t *testing.T) {
	a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))
	jpeg := testPhoto(t, true)
	raw := testPhotoNamed(t, "DSC002.ARW")
	both := tagWrite{sidecar: true, jpeg: true}

	tests := []struct {
		name    string
		photo   model.Photo
		written model.Tags
		manual  bool
		want    string
	}{
		{
			name:    "tags a stock site would take",
			photo:   jpeg,
			written: uploadableTags(),
			manual:  true,
		},
		{
			name:    "tags that are not ready",
			photo:   jpeg,
			written: typedTags(),
			manual:  true,
			want:    jpeg.Name + " was left alone: 2 keywords, expected 50",
		},
		{
			name:    "a save nobody asked for stays quiet",
			photo:   jpeg,
			written: typedTags(),
		},
		{
			name:    "a photo that is no JPEG has nothing to say",
			photo:   raw,
			written: typedTags(),
			manual:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &tagsSession{app: a, photo: tt.photo}
			plan := session.writePlan(tt.written, both, tt.manual)
			assert.Equal(t, tt.want, session.skipNote(tt.written, both, plan, tt.manual))
		})
	}
}
