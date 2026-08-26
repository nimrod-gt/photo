package app

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"photo/internal/core/model"
)

func copiedTags() model.Tags {
	return model.Tags{
		Title:    typedTags().Title,
		Keywords: typedTags().Keywords,
		Place:    model.Place{Location: "Lisbon"},
		Concept:  testConcept,
	}
}

// The buttons of a Fyne confirm sit inside its own widgets, which a container
// walk alone does not reach.
func findButton(obj fyne.CanvasObject, label string) *widget.Button {
	switch o := obj.(type) {
	case *widget.Button:
		if o.Text == label {
			return o
		}
	case *fyne.Container:
		for _, child := range o.Objects {
			if found := findButton(child, label); found != nil {
				return found
			}
		}
	case fyne.Widget:
		for _, child := range test.WidgetRenderer(o).Objects() {
			if found := findButton(child, label); found != nil {
				return found
			}
		}
	}
	return nil
}

func confirmButton(t *testing.T, a *Application, label string) *widget.Button {
	t.Helper()
	top := a.mainWindow.Window().Canvas().Overlays().Top()
	require.NotNil(t, top, "no confirm on screen")
	button := findButton(top, label)
	require.NotNilf(t, button, "the confirm has no %q button", label)
	return button
}

func TestTagsClipboardCopy(t *testing.T) {
	t.Run("takes the title and the keywords and leaves the rest", func(t *testing.T) {
		a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))
		session := a.openTestTagsDialog(t, testPhoto(t, false))
		typeIntoDialog(session, copiedTags())

		session.copyTags()

		require.True(t, a.tagsCopied)
		assert.Equal(t, typedTags(), a.tagsCopy, "the place and the note stayed with their photo")
	})

	t.Run("a dialog with no tags leaves the clipboard as it was", func(t *testing.T) {
		a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))
		filled := a.openTestTagsDialog(t, testPhoto(t, false))
		typeIntoDialog(filled, typedTags())
		filled.copyTags()

		empty := a.openTestTagsDialog(t, testPhotoNamed(t, "DSC002.JPG"))
		typeIntoDialog(empty, model.Tags{Concept: testConcept})
		empty.copyTags()

		assert.Equal(t, typedTags(), a.tagsCopy, "a copy of nothing emptied the clipboard")
	})
}

func TestTagsClipboardPaste(t *testing.T) {
	t.Run("empty fields take the tags with no question asked", func(t *testing.T) {
		a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))
		a.tagsCopy, a.tagsCopied = typedTags(), true
		session := a.openTestTagsDialog(t, testPhoto(t, false))

		session.pasteTags()

		assert.Equal(t, typedTags(), session.dialog.Tags())
		assert.Nil(t, a.mainWindow.Window().Canvas().Overlays().Top(), "an empty dialog was asked about")
	})

	t.Run("the concept and the location of the photo pasted into stay", func(t *testing.T) {
		a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))
		a.tagsCopy, a.tagsCopied = typedTags(), true
		session := a.openTestTagsDialog(t, testPhoto(t, false))
		typeIntoDialog(session, model.Tags{Place: model.Place{Location: "Porto"}, Concept: testConcept})

		session.pasteTags()

		assert.Equal(t, "Porto", session.dialog.Location())
		assert.Equal(t, testConcept, session.dialog.Concept())
	})

	t.Run("tags already on the dialog are replaced only once the confirm is answered", func(t *testing.T) {
		a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))
		a.tagsCopy, a.tagsCopied = typedTags(), true
		session := a.openTestTagsDialog(t, testPhoto(t, false))
		typeIntoDialog(session, existingSidecar())

		session.pasteTags()
		assert.Equal(t, existingSidecar(), session.dialog.Tags(), "the paste did not wait to be confirmed")

		test.Tap(confirmButton(t, a, "Paste"))
		assert.Equal(t, typedTags(), session.dialog.Tags())
	})

	t.Run("a cancelled confirm leaves the tags alone", func(t *testing.T) {
		a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))
		a.tagsCopy, a.tagsCopied = typedTags(), true
		session := a.openTestTagsDialog(t, testPhoto(t, false))
		typeIntoDialog(session, existingSidecar())

		session.pasteTags()
		test.Tap(confirmButton(t, a, "Cancel"))

		assert.Equal(t, existingSidecar(), session.dialog.Tags())
	})

	t.Run("nothing copied yet pastes nothing", func(t *testing.T) {
		a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))
		session := a.openTestTagsDialog(t, testPhoto(t, false))

		session.pasteTags()

		assert.True(t, session.dialog.Tags().IsEmpty())
		assert.Nil(t, a.mainWindow.Window().Canvas().Overlays().Top())
	})

	// The confirm owns the keyboard while it is up, so the chord underneath it
	// cannot stack a second one.
	t.Run("a chord repeated over the confirm does nothing", func(t *testing.T) {
		a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))
		a.tagsCopy, a.tagsCopied = typedTags(), true
		session := a.openTestTagsDialog(t, testPhoto(t, false))
		typeIntoDialog(session, existingSidecar())
		session.dialog.Show()
		t.Cleanup(session.dialog.Hide)

		session.pasteTags()
		overlays := len(a.mainWindow.Window().Canvas().Overlays().List())
		session.pasteTags()
		session.copyTags()

		assert.Len(t, a.mainWindow.Window().Canvas().Overlays().List(), overlays)
		assert.Equal(t, typedTags(), a.tagsCopy, "the copy underneath the confirm ran")
	})
}
