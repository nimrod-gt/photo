package app

import (
	"errors"
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
	assert.Equal(t, rewrittenNote+"; "+placeDroppedNote+"; "+conceptDroppedNote,
		writeNote(imaging.StockWrite{Rewritten: true, PlaceDropped: true, ConceptDropped: true}))
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

// The note is what a generation is made from, so it is worth a sidecar before
// there are any tags to go with it.
func TestTagsSessionCloseWithAConceptAlone(t *testing.T) {
	a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))
	photo := testPhoto(t, true)
	session := a.openTestTagsDialog(t, photo)

	typeIntoDialog(session, model.Tags{Concept: testConcept})
	session.close()

	require.Eventually(t, func() bool {
		written, err := imaging.ReadSidecar(model.SidecarPath(photo.RAWPath))
		return err == nil && written.Concept == testConcept
	}, time.Second, 5*time.Millisecond, "the sidecar was never written")

	reopened := a.openTestTagsDialog(t, photo)
	reopened.seed()

	assert.Equal(t, testConcept, reopened.dialog.Concept(),
		"the dialog opens on what the last one saved")
}

func existingSidecar() model.Tags {
	return model.Tags{Title: "What the file already says.", Keywords: []string{"lake", "morning"}}
}

func writeTestSidecar(t *testing.T, photo model.Photo, written model.Tags) {
	t.Helper()
	require.NoError(t, imaging.WriteSidecar(model.SidecarPath(photo.RAWPath), written))
}

func awaitSidecar(t *testing.T, photo model.Photo, want model.Tags) {
	t.Helper()
	path := model.SidecarPath(photo.RAWPath)
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
