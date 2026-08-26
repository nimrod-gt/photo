package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"photo/internal/core/imaging"
	"photo/internal/core/model"
)

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
		{name: "a concept the user typed", tags: model.Tags{Concept: "tram 28 seen head-on"}},
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

	typeIntoDialog(session, model.Tags{Concept: "tram 28 seen head-on"})
	session.close()

	require.Eventually(t, func() bool {
		written, err := imaging.ReadSidecar(model.SidecarPath(photo.RAWPath))
		return err == nil && written.Concept == "tram 28 seen head-on"
	}, time.Second, 5*time.Millisecond, "the sidecar was never written")

	reopened := a.openTestTagsDialog(t, photo)
	reopened.seed()

	assert.Equal(t, "tram 28 seen head-on", reopened.dialog.Concept(),
		"the dialog opens on what the last one saved")
}
