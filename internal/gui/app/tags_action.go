package app

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
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
		ShowSave:   a.saveButtonVisible(),
	}, a.mainWindow.Window(), ui.TagsDialogCallbacks{
		OnEscape:     a.handleCancel,
		OnGenerate:   session.generate,
		OnStopRun:    session.stopRun,
		OnBackground: session.background,
		OnCopyTitle: func() {
			a.fyneApp.Clipboard().SetContent(session.dialog.Tags().Title)
			a.notifier.ShowNotification("Title copied to clipboard")
		},
		OnCopyKeywords: func() {
			a.fyneApp.Clipboard().SetContent(session.dialog.Tags().KeywordLine())
			a.notifier.ShowNotification("Keywords copied to clipboard")
		},
		OnCopyTags:  session.copyTags,
		OnPasteTags: session.pasteTags,
		OnSave:      session.save,
		OnClose:     session.close,
	})

	a.dialogs.openSelfClosing(dialogTags, session.dialog, session.escape)
	session.dialog.Show()
	session.seed()
	// A run that is still going fills the result fields itself when it lands.
	// The read still happens underneath: it only ever fills fields that are
	// empty, and it is the only thing that knows what the file already holds -
	// which is what the dialog falls back to when the run fails.
	if typed, ok := a.tagRuns.attach(session); ok {
		session.dialog.Generating()
		// Whatever the dialog that closed over the run held is newer than the
		// file, and the run gives it back rather than writing it: with a dialog
		// listening again, a run that fails only fills the status line.
		if !nothingToWrite(typed) {
			session.dialog.RestoreTags(typed)
		}
	}
	session.prefill()
}

type tagsSession struct {
	app    *Application
	photo  model.Photo
	dialog *ui.TagsDialog
	saved  model.Tags
	taken  time.Time
	// known says the session has been told what the files already hold. Until
	// it has, everything on the dialog was typed into fields that were blank
	// for want of a read rather than for want of tags, so a save from it adds
	// to what is there instead of taking its place - see completed.
	known bool
}

// Tags whole enough to show are already in the cache whenever the photo was
// loaded, so the dialog opens filled instead of blank until a read lands.
func (s *tagsSession) seed() {
	info, ok := s.app.imageProvider.PeekStockInfo(s.photo.ImagePath)
	if !ok {
		return
	}
	s.known = true
	s.taken = info.Taken
	// Tags the cache holds for want of a sidecar are on screen and nowhere
	// else, so nothing here counts as saved and closing over them writes.
	if !s.app.tagsUnsaved.has(s.photo.ImagePath) {
		s.saved = info.Tags
	}
	s.dialog.SetPhotoInfo(info.Tags, info.Taken)
}

// The shooting date is read from the file and never edited here, so what was
// read for the photo is kept beside the tags the app itself wrote. A save that
// beat the read to it has no date of its own and the cache keeps the one it
// already holds.
func (a *Application) storeStock(path string, written model.Tags, taken time.Time, sidecar bool) {
	a.imageProvider.StoreStockInfo(path, imaging.StockInfo{Tags: written, Taken: taken})
	a.tagsUnsaved.mark(path, !sidecar)
	a.setTagsIfCurrent(path, written)
}

// The cache means "this is what the photo has", which is what a dialog opening
// on it takes for the contents of the sidecar. With the sidecar left to the
// Save button the cache runs ahead of the file, and the photos it does so for
// are remembered here: the next dialog is then told to write them rather than
// to believe them.
//
// Locked rather than left to the UI goroutine, because a save marks the photo
// from the worker goroutine that wrote the file.
type unsavedTags struct {
	mu    sync.Mutex
	paths map[string]struct{}
}

func (u *unsavedTags) mark(path string, unsaved bool) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if !unsaved {
		delete(u.paths, path)
		return
	}
	if u.paths == nil {
		u.paths = make(map[string]struct{})
	}
	u.paths[path] = struct{}{}
}

func (u *unsavedTags) has(path string) bool {
	u.mu.Lock()
	defer u.mu.Unlock()

	_, ok := u.paths[path]
	return ok
}

func (s *tagsSession) generate() {
	s.app.tagRuns.start(s, tags.Request{
		Photo:      s.photo,
		Location:   s.dialog.Location(),
		Concept:    s.dialog.Concept(),
		Notes:      s.dialog.Notes(),
		Editorial:  s.dialog.Editorial(),
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
	s.app.storeStock(s.photo.ImagePath, generated, s.taken, false)
	s.autoSave(generated)
}

// Escape means cancel throughout the app, and what it cancels here is the run
// rather than the dialog: the tags it would have brought are gone either way,
// but everything typed to ask for them stays, so a second attempt starts where
// the first one did. With no run going there is nothing left to cancel but the
// dialog itself.
func (s *tagsSession) escape() {
	if s.dialog.IsGenerating() {
		s.stopRun()
		return
	}
	s.close()
}

func (s *tagsSession) stopRun() {
	s.app.tagRuns.cancel(s.photo.ImagePath)
	s.dialog.StopGenerating()
}

func (s *tagsSession) background() {
	s.app.tagRuns.background(s.photo.ImagePath)
	s.close()
}

// The sidecar is the store of the photo, pair or no pair; the JPEG is the photo
// itself, which is why the settings hold the two apart.
type tagWrite struct {
	sidecar bool
	jpeg    bool
}

func (w tagWrite) none() bool {
	return !w.sidecar && !w.jpeg
}

// A JPEG is only written with tags a stock site would take: that file is the
// photo itself, and the status line already spells out what is missing.
func (w tagWrite) forPhoto(photo model.Photo, written model.Tags) tagWrite {
	w.jpeg = w.jpeg && photo.IsJPEG() && len(written.Problems()) == 0
	return w
}

func (a *Application) autoWrite() tagWrite {
	return tagWrite{sidecar: a.autoSaveXMP, jpeg: a.autoSaveJPEG}
}

// What the settings let happen on its own - right after a run and again on
// close, once the user has edited the tags.
// Emptying both fields is an edit like any other and clears the tags in the
// files; only a photo that never had any is left without a sidecar.
// A generation that is still going writes the same files when it lands, so the
// dialog closing over it - backgrounded, or clicked away - hands its fields to
// the run instead of racing it with a write of its own; the run keeps them if
// it brings nothing better. The handover happens whatever the settings say: a
// dialog reopened over the run puts those fields back on screen.
func (s *tagsSession) autoSave(written model.Tags) {
	if s.app.tagRuns.takeOver(s.photo.ImagePath, written, s.known) {
		return
	}
	s.write(written, s.app.autoWrite(), false)
}

// The Save button is the one way to write files the settings leave alone, so it
// asks for both and says nothing about tags that are already there: with the
// sidecar saved by hand, what the dialog was filled with is what the user means
// to keep.
func (s *tagsSession) save() {
	s.write(s.dialog.Tags(), tagWrite{sidecar: true, jpeg: true}, true)
}

// Tags the files already hold are left alone unless the save was asked for by
// hand, and the JPEG goes by the same rule as the sidecar: rewriting it over
// nothing costs a read of the whole file, and on the EXIF fallback path a
// rewrite of it, every time a dialog is opened and closed again.
func (s *tagsSession) writePlan(written model.Tags, want tagWrite, manual bool) tagWrite {
	// A photo that never had tags is left without a sidecar even when the save
	// was asked for by hand: there would be nothing in the file.
	blank := nothingToWrite(written) && nothingToWrite(s.saved)
	changed := manual || !written.Equal(s.saved)
	return tagWrite{
		sidecar: want.sidecar && !blank && changed,
		jpeg:    want.jpeg && changed,
	}.forPhoto(s.photo, written)
}

// A save asked for by hand that leaves the JPEG behind says why, and says it
// alongside what was written rather than before it: the notifier holds one
// message at a time and the write landing afterwards would push this one off.
func (s *tagsSession) skipNote(written model.Tags, want, plan tagWrite, manual bool) string {
	if !manual || !want.jpeg || !s.photo.IsJPEG() || plan.jpeg {
		return ""
	}
	return s.photo.Name + " was left alone: " + strings.Join(written.Problems(), "; ")
}

// Both files are written by one save rather than by two racing each other: the
// notifier holds one message at a time, and the second of two writes announcing
// themselves apart would push the first off the screen.
// The tags count as saved before the write finishes, so a second close does not
// repeat it, and a failed write puts the previous ones back so the next close
// tries again.
func (s *tagsSession) write(written model.Tags, want tagWrite, manual bool) {
	plan := s.writePlan(written, want, manual)
	skipped := s.skipNote(written, want, plan, manual)
	if plan.none() {
		if len(skipped) != 0 {
			s.app.notifier.ShowWarning(skipped)
		}
		return
	}
	previous := s.saved
	s.saved = written
	taken := s.taken
	known := s.known
	s.app.saveTags(written, writeTarget(s.photo, plan), func(saved model.Tags) (string, error) {
		saved, note, err := s.app.writeTagFiles(s.photo, saved, plan, known)
		if err != nil {
			return "", err
		}
		if len(note) == 0 {
			note = skipped
		}
		s.app.storeStock(s.photo.ImagePath, saved, taken, plan.sidecar)
		return note, nil
	}, func() {
		s.saved = previous
		if !plan.sidecar {
			return
		}
		// A generated run cached its tags before this write; leaving them there
		// would tell the next dialog they are saved and stop it from writing
		// them again, so the entry goes and the file is read instead. A write
		// that was only for the JPEG keeps the cache: the sidecar it would be
		// read back from was never asked to hold these tags.
		s.app.imageProvider.Forget(s.photo.ImagePath)
		s.app.tagsUnsaved.mark(s.photo.ImagePath, false)
	})
}

func writeTarget(photo model.Photo, plan tagWrite) string {
	var targets []string
	if plan.sidecar {
		targets = append(targets, filepath.Base(photo.SidecarPath()))
	}
	if plan.jpeg {
		targets = append(targets, photo.Name)
	}
	return strings.Join(targets, " and ")
}

// What the sidecar already holds is folded in whichever file is written: the
// fields may have been typed into a dialog that never read it, and the JPEG
// would lose them as readily as the sidecar. The note comes back only from a
// JPEG write - the sidecar takes everything and has nothing to remark on.
func (a *Application) writeTagFiles(photo model.Photo, written model.Tags, plan tagWrite, known bool) (model.Tags, string, error) {
	saved, err := completed(photo.SidecarPath(), written, known)
	if err != nil {
		return saved, "", err
	}
	if plan.sidecar {
		if err := imaging.WriteSidecar(photo.SidecarPath(), saved); err != nil {
			return saved, "", err
		}
	}
	if !plan.jpeg {
		return saved, "", nil
	}
	write, err := a.exifService.WriteStockTags(photo.ImagePath, saved)
	if err != nil {
		return saved, "", err
	}
	return saved, writeNote(write), nil
}

// A dialog closed before the read of its photo landed knows nothing of what the
// sidecar already holds: every field on it stood empty for want of that read
// rather than for want of tags, so whatever was typed into one was added to
// nothing. The save adds to the file in the same way, because emptying a field
// is what puts a save in place of what is there, and no field on such a dialog
// can have been emptied.
func completed(path string, written model.Tags, known bool) (model.Tags, error) {
	if known {
		return written, nil
	}
	existing, err := imaging.ReadSidecar(path)
	if err != nil {
		return written, fmt.Errorf("reading %s before saving over it: %w", filepath.Base(path), err)
	}
	return imaging.FillMissing(written, existing), nil
}

// Tags.IsEmpty ignores the place, the concept, the notes and the editorial mark
// on purpose - none of the four is a result the generator produced - but what
// the user filled in there is worth a sidecar of its own, with or without tags
// to go with it.
func nothingToWrite(tags model.Tags) bool {
	return tags.IsEmpty() && tags.Place.IsEmpty() &&
		len(strings.TrimSpace(tags.Concept)) == 0 && len(strings.TrimSpace(tags.Notes)) == 0 &&
		tags.Editorial.IsEmpty()
}

// The JPEG is only replaced when its XMP packet has no room for the tags. A
// camera keeps a database of the files it wrote and refuses to display a photo
// whose file changed under it until the user rebuilds that database, so that
// case is spelled out instead of passing as a plain success.
const rewrittenNote = "the file was rewritten, a Sony camera shows it again after Recover Image DB"

// No EXIF tag holds a place, a concept, the notes or an editorial mark, so the
// fallback the note above describes carries the title and the keywords and
// leaves those four behind.
const placeDroppedNote = "the location was not written: the XMP packet had no room and the EXIF has no field for a place"

const conceptDroppedNote = "the concept was not written: the XMP packet had no room and the EXIF has no field for it"

const notesDroppedNote = "the notes were not written: the XMP packet had no room and the EXIF has no field for them"

const editorialDroppedNote = "the editorial mark was not written: the XMP packet had no room and the EXIF has no field for it"

func writeNote(write imaging.StockWrite) string {
	var notes []string
	if write.Rewritten {
		notes = append(notes, rewrittenNote)
	}
	if write.PlaceDropped {
		notes = append(notes, placeDroppedNote)
	}
	if write.ConceptDropped {
		notes = append(notes, conceptDroppedNote)
	}
	if write.NotesDropped {
		notes = append(notes, notesDroppedNote)
	}
	if write.EditorialDropped {
		notes = append(notes, editorialDroppedNote)
	}
	return strings.Join(notes, "; ")
}

func (s *tagsSession) close() {
	s.autoSave(s.dialog.Tags())
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
	if s.known {
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
			// A read that failed says nothing about the files, so the session
			// stays in the dark and keeps adding to them rather than replacing.
			s.known = err == nil
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
				a.notifier.ShowWarning("Tags saved to " + target + " - " + note)
				return
			}
			a.notifier.ShowNotification("Tags saved to " + target)
		})
	}()
}
