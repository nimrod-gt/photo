package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
	assert.Equal(t, rewrittenNote+"; "+placeDroppedNote,
		writeNote(imaging.StockWrite{Rewritten: true, PlaceDropped: true}))
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
