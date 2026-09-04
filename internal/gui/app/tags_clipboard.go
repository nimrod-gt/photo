package app

import (
	"fyne.io/fyne/v2/dialog"

	"photo/internal/core/model"
)

// The tags travel from one photo's dialog to another's through the app itself,
// so closing the dialog they were taken from does not lose them. Both actions
// are dialog callbacks and run on the UI goroutine, which is the only one that
// ever touches the clipboard.
//
// A Fyne popup - the date calendar, or the paste confirm - stacks its own
// overlay on top and owns the keyboard while it is up, the same reason Escape
// checks for one.
func (s *tagsSession) copyTags() {
	if s.app.foreignOverlayOnTop() {
		return
	}
	copied := s.dialog.Tags()
	if copied.IsEmpty() {
		s.app.notifier.ShowWarning("No tags to copy")
		return
	}
	// The place, the concept, the notes and the editorial mark belong to the
	// photo they were typed for; only what a generation produced travels to
	// another one.
	s.app.tagsCopy = model.Tags{Title: copied.Title, Keywords: copied.Keywords}
	s.app.tagsCopied = true
	s.app.notifier.ShowNotification("Tags copied")
}

// Nothing is written here: the fields are filled and the paths that already
// write them do the rest - close for the sidecar, Save JPEG for the packet.
func (s *tagsSession) pasteTags() {
	if s.app.foreignOverlayOnTop() {
		return
	}
	if !s.app.tagsCopied {
		s.app.notifier.ShowWarning("No tags copied yet")
		return
	}
	if !s.dialog.HasTags() {
		s.dialog.PasteTags(s.app.tagsCopy)
		return
	}
	// The confirm goes through Fyne directly rather than through showConfirm:
	// dialogManager holds one dialog and that one is the Tags dialog underneath.
	confirm := dialog.NewConfirm("Paste Tags", "Replace the title and keywords with the copied ones?",
		func(confirmed bool) {
			if !confirmed {
				return
			}
			s.dialog.PasteTags(s.app.tagsCopy)
		}, s.app.mainWindow.Window())
	confirm.SetConfirmText("Paste")
	confirm.SetDismissText("Cancel")
	confirm.Show()
}
