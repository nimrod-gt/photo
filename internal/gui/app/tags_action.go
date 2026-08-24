package app

import (
	"context"
	"log"
	"path/filepath"
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
	ctx, cancel := context.WithCancel(context.Background())
	// The date the photo was taken is settled when its image is read, so the
	// dialog is built with it and never shows another one first.
	taken, _ := a.imageProvider.PeekStockDate(photo.ImagePath)
	session := &tagsSession{app: a, photo: photo, prefs: prefs, taken: taken}

	session.dialog = ui.NewTagsDialog(ui.TagsDialogOptions{
		Filename:   photo.Name,
		ClaudePath: prefs.String("claudePath"),
		Date:       taken,
		IsJPEG:     photo.IsJPEG(),
	}, a.mainWindow.Window(), ui.TagsDialogCallbacks{
		OnEscape:   a.handleCancel,
		OnGenerate: func() { session.generate(ctx) },
		OnCopyTitle: func() {
			a.fyneApp.Clipboard().SetContent(session.dialog.Tags().Title)
			a.mainWindow.ShowNotification("Title copied to clipboard")
		},
		OnCopyKeywords: func() {
			a.fyneApp.Clipboard().SetContent(session.dialog.Tags().KeywordLine())
			a.mainWindow.ShowNotification("Keywords copied to clipboard")
		},
		OnSaveJPEG: session.saveJPEG,
		OnClose:    func() { session.close(cancel) },
	})

	a.dialogs.open(dialogTags, session.dialog, func() { session.close(cancel) })
	session.dialog.Show()
	session.seed()
	session.prefill()
}

type tagsSession struct {
	app    *Application
	photo  model.Photo
	prefs  fyne.Preferences
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
}

func (s *tagsSession) generate(ctx context.Context) {
	req := tags.Request{Photo: s.photo, Notes: s.dialog.Notes(), ClaudePath: s.dialog.ClaudePath()}

	go func() {
		generated, err := s.app.tagger.Generate(ctx, req)
		fyne.Do(func() {
			if !s.app.dialogs.isCurrent(s.dialog) {
				return
			}
			if err != nil {
				log.Println("Failed to generate tags:", err)
				s.dialog.Fail(err)
				return
			}
			// Only a path that produced tags is remembered, and an empty one
			// clears the preference: a stored path short-circuits the search
			// for the binary, so a typo saved eagerly would disable it for good.
			s.prefs.SetString("claudePath", req.ClaudePath)
			s.dialog.SetTags(generated)
			s.storeStock(generated, s.taken)
			s.saveSidecar(generated)
		})
	}()
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
	if !s.photo.HasRAW() || written.Equal(s.saved) || (written.IsEmpty() && s.saved.IsEmpty()) {
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

// The JPEG is only replaced when its XMP packet has no room for the tags. A
// camera keeps a database of the files it wrote and refuses to display a photo
// whose file changed under it until the user rebuilds that database, so that
// case is spelled out instead of passing as a plain success.
const rewrittenNote = "the file was rewritten, a Sony camera shows it again after Recover Image DB"

func (s *tagsSession) saveJPEG() {
	taken := s.taken
	s.app.saveTags(s.dialog.Tags(), s.photo.Name, func(saved model.Tags) (string, error) {
		rewritten, err := s.app.exifService.WriteStockTags(s.photo.ImagePath, saved)
		if err != nil {
			return "", err
		}
		s.storeStock(saved, taken)
		if !rewritten {
			return "", nil
		}
		return rewrittenNote, nil
	}, nil)
}

func (s *tagsSession) close(cancel context.CancelFunc) {
	cancel()
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
