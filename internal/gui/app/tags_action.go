package app

import (
	"log"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"

	"photo/internal/core/imaging"
	"photo/internal/core/model"
	"photo/internal/core/tags"
	"photo/internal/gui/ui"
)

func (a *Application) handleTags() {
	if a.shortcutsBlocked() {
		return
	}
	photo, ok := a.navigator.Current()
	if !ok {
		return
	}

	prefs := a.fyneApp.Preferences()
	// The date the photo was taken is settled when its image is read, so the
	// dialog is built with it and never shows another one first.
	taken, _ := a.imageProvider.PeekStockDate(photo.ImagePath)
	session := &tagsSession{app: a, photo: photo, taken: taken}

	session.dialog = ui.NewTagsDialog(ui.TagsDialogOptions{
		Filename:   photo.Name,
		ClaudePath: prefs.String("claudePath"),
		Date:       taken,
		IsJPEG:     photo.IsJPEG(),
	}, a.mainWindow.Window(), ui.TagsDialogCallbacks{
		OnEscape:     a.handleCancel,
		OnGenerate:   session.generate,
		OnStopRun:    session.cancelRun,
		OnBackground: session.background,
		OnCopyTitle: func() {
			a.fyneApp.Clipboard().SetContent(session.dialog.Tags().Title)
			a.mainWindow.ShowNotification("Title copied to clipboard")
		},
		OnCopyKeywords: func() {
			a.fyneApp.Clipboard().SetContent(session.dialog.Tags().KeywordLine())
			a.mainWindow.ShowNotification("Keywords copied to clipboard")
		},
		OnSaveJPEG: session.saveJPEG,
		OnClose:    session.close,
	})

	a.dialogs.open(dialogTags, session.dialog, session.cancelRun)
	session.dialog.Show()
	session.seed()
	// A run that is still going fills the result fields itself when it lands.
	// The read still happens underneath: it only ever fills fields that are
	// empty, and it is the only thing that knows what the file already holds -
	// which is what the dialog falls back to when the run fails.
	if a.tagRuns.attach(session) {
		session.dialog.Generating()
	}
	session.prefill()
}

type tagsSession struct {
	app    *Application
	photo  model.Photo
	dialog *ui.TagsDialog
	saved  model.Tags
	taken  time.Time
	seeded bool
}

// Tags whole enough to show are already in the cache whenever the photo was
// loaded, so the dialog opens filled instead of blank until a read lands.
func (s *tagsSession) seed() {
	info, ok := s.app.imageProvider.PeekStockInfo(s.photo.ImagePath)
	if !ok {
		return
	}
	s.seeded = true
	s.saved = info.Tags
	s.taken = info.Taken
	s.dialog.SetPhotoInfo(info.Tags, info.Taken)
}

// The shooting date is read from the file and never edited here, so what was
// read for the photo is kept beside the tags the app itself wrote. A save that
// beat the read to it has no date of its own and the cache keeps the one it
// already holds.
func (s *tagsSession) storeStock(written model.Tags, taken time.Time) {
	s.app.imageProvider.StoreStockInfo(s.photo.ImagePath, imaging.StockInfo{Tags: written, Taken: taken})
	s.app.setTagsIfCurrent(s.photo.ImagePath, written)
}

func (s *tagsSession) generate() {
	s.app.tagRuns.start(s, tags.Request{
		Photo:      s.photo,
		Notes:      s.dialog.Notes(),
		Location:   s.dialog.Location(),
		ClaudePath: s.dialog.ClaudePath(),
	})
}

// Reached only while this dialog is still the one on screen; a run that lost it
// reports itself instead, sidecar included.
func (s *tagsSession) runFinished(generated model.Tags, err error) {
	if err != nil {
		s.dialog.Fail(err)
		return
	}
	s.dialog.SetTags(generated)
	s.storeStock(generated, s.taken)
	s.saveSidecar(generated)
}

// Escape means cancel throughout the app, so it does here what the Cancel
// button does: the run dies with the dialog. Background is the way to keep one.
func (s *tagsSession) cancelRun() {
	s.app.tagRuns.cancel(s.photo.ImagePath)
	s.close()
}

func (s *tagsSession) background() {
	s.app.tagRuns.background(s.photo.ImagePath)
	s.close()
}

// B is the Blue label everywhere else, and a dialog on screen blocks that
// anyway; over a running generation it is the Background button instead, so the
// key and the button say the same thing.
func (a *Application) handleBlue() {
	if session, ok := a.tagRuns.visible(); ok {
		if !a.foreignOverlayOnTop() {
			session.background()
		}
		return
	}
	a.handleColorToggle(model.ColorBlue)
}

// The sidecar belongs to us alone, so it is written without asking - right
// after a run and again on close, once the user has edited the tags. Writing
// into the JPEG stays behind its button, because that file is the photo itself.
// Emptying both fields is an edit like any other and clears the tags in the
// sidecar, the way it clears them in the JPEG; only a photo that never had any
// is left without a sidecar.
// The tags count as saved before the write finishes, so a second close does not
// repeat it, and a failed write puts the previous ones back so the next close
// tries again.
func (s *tagsSession) saveSidecar(written model.Tags) {
	if !s.photo.HasRAW() || written.Equal(s.saved) || (nothingToWrite(written) && nothingToWrite(s.saved)) {
		return
	}
	previous := s.saved
	s.saved = written
	taken := s.taken
	path := model.SidecarPath(s.photo.RAWPath)
	s.app.saveTags(written, filepath.Base(path), func(saved model.Tags) (string, error) {
		if err := imaging.WriteSidecar(path, saved); err != nil {
			return "", err
		}
		s.storeStock(saved, taken)
		return "", nil
	}, func() {
		s.saved = previous
		// A generated run cached its tags before this write; leaving them there
		// would tell the next dialog they are saved and stop it from writing
		// them again, so the entry goes and the file is read instead.
		s.app.imageProvider.Forget(s.photo.ImagePath)
	})
}

// Tags.IsEmpty ignores the place on purpose - a place alone is not a result the
// generator produced - but a location the user typed is worth a sidecar of its
// own, with or without tags to go with it.
func nothingToWrite(tags model.Tags) bool {
	return tags.IsEmpty() && tags.Place.IsEmpty()
}

// The JPEG is only replaced when its XMP packet has no room for the tags. A
// camera keeps a database of the files it wrote and refuses to display a photo
// whose file changed under it until the user rebuilds that database, so that
// case is spelled out instead of passing as a plain success.
const rewrittenNote = "the file was rewritten, a Sony camera shows it again after Recover Image DB"

// No EXIF tag holds a place, so the fallback the note above describes carries
// the title and the keywords and leaves the location behind.
const placeDroppedNote = "the location was not written: the XMP packet had no room and the EXIF has no field for a place"

func (s *tagsSession) saveJPEG() {
	taken := s.taken
	s.app.saveTags(s.dialog.Tags(), s.photo.Name, func(saved model.Tags) (string, error) {
		write, err := s.app.exifService.WriteStockTags(s.photo.ImagePath, saved)
		if err != nil {
			return "", err
		}
		s.storeStock(saved, taken)
		return writeNote(write), nil
	}, nil)
}

func writeNote(write imaging.StockWrite) string {
	var notes []string
	if write.Rewritten {
		notes = append(notes, rewrittenNote)
	}
	if write.PlaceDropped {
		notes = append(notes, placeDroppedNote)
	}
	return strings.Join(notes, "; ")
}

func (s *tagsSession) close() {
	s.saveSidecar(s.dialog.Tags())
	if s.app.dialogs.isCurrent(s.dialog) {
		s.app.dialogs.closed()
	}
	s.dialog.Hide()
}

// The tags the dialog is filled with count as saved, so closing it without
// touching them writes nothing. Comparing against the sidecar instead would
// overwrite it with the EXIF of the JPEG whenever the two disagree.
// A run that finished before this read - the file may sit on a slow volume -
// already wrote what the sidecar holds, so its tags are the ones kept.
func (s *tagsSession) prefill() {
	if s.seeded {
		return
	}
	go func() {
		info, err := s.app.imageProvider.StockInfo(s.photo)
		if err != nil {
			log.Printf("Failed to read tags of %s: %v", s.photo.Name, err)
		}
		fyne.Do(func() {
			if !s.app.dialogs.isCurrent(s.dialog) {
				return
			}
			if s.saved.IsEmpty() {
				s.saved = info.Tags
			}
			// A read that failed carries no date, and the session already holds
			// the one the cache had when the dialog opened.
			if !info.Taken.IsZero() {
				s.taken = info.Taken
			}
			s.dialog.SetPhotoInfo(info.Tags, info.Taken)
		})
	}()
}

// A save may come back with a note about how it had to be done; it is shown as
// a warning in place of the plain confirmation.
func (a *Application) saveTags(written model.Tags, target string, save func(model.Tags) (string, error), failed func()) {
	go func() {
		note, err := save(written)
		fyne.Do(func() {
			if err != nil {
				if failed != nil {
					failed()
				}
				a.showError("Failed to save tags to "+target, err)
				return
			}
			if len(note) != 0 {
				a.mainWindow.ShowWarning("Tags saved to " + target + " - " + note)
				return
			}
			a.mainWindow.ShowNotification("Tags saved to " + target)
		})
	}()
}
