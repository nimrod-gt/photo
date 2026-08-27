package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"photo/internal/core/imaging"
	"photo/internal/core/model"
	"photo/internal/core/tags"
	"photo/internal/gui/ui"
)

// The generation answers only when the test lets it, so everything the runner
// does while a run is in flight can be asserted from the test goroutine alone.
type heldTagger struct {
	started chan struct{}
	release chan struct{}
	tags    model.Tags
	err     error
}

func newHeldTagger(generated model.Tags, err error) *heldTagger {
	return &heldTagger{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
		tags:    generated,
		err:     err,
	}
}

func (h *heldTagger) Generate(ctx context.Context, _ tags.Request) (model.Tags, error) {
	h.started <- struct{}{}
	select {
	case <-h.release:
	case <-ctx.Done():
		return model.Tags{}, ctx.Err()
	}
	return h.tags, h.err
}

// The Fyne test driver runs fyne.Do on the caller's goroutine, so a run reports
// itself on its own goroutine right after the generation answers. What the
// report stored in the cache is read back under the provider's lock, and that
// is what makes the rest of the report - the dialog it filled in - safe to look
// at from here.
func (h *heldTagger) answerWithTags(t *testing.T, a *Application, path string) imaging.StockInfo {
	t.Helper()
	close(h.release)
	var info imaging.StockInfo
	require.Eventually(t, func() bool {
		var ok bool
		info, ok = a.imageProvider.PeekStockInfo(path)
		return ok
	}, time.Second, 5*time.Millisecond, "the run stored nothing")
	return info
}

// The wait ends when the generation answers, which is one step short of the
// report it hands to fyne.Do, so the window afterwards is a bound rather than a
// proof. What proves the run is dead is asserted by the caller: it is gone from
// the registry and it wrote no file.
func (h *heldTagger) answerWithNothing(t *testing.T, a *Application, r *tagRunner, path string) {
	t.Helper()
	close(h.release)
	r.running.Wait()
	assert.Never(t, func() bool {
		_, ok := a.imageProvider.PeekStockInfo(path)
		return ok
	}, 200*time.Millisecond, 5*time.Millisecond, "the run stored tags it was told to drop")
}

// The Fyne test driver keeps global state, so these tests share an app and must
// not run in parallel.
func newTestApplication(t *testing.T, generator tagGenerator) *Application {
	t.Helper()
	fyneApp := test.NewTempApp(t)
	a := New()
	a.fyneApp = fyneApp
	a.tagger = generator
	a.actionPanel = ui.NewActionPanel(ui.ActionPanelCallbacks{})
	a.fileBrowser = ui.NewFileBrowser(a.scanner, a.imageProvider, a.colorService, ui.FileBrowserCallbacks{})
	a.viewer = ui.NewViewer(ui.ViewerCallbacks{})
	a.gridViewer = ui.NewGridViewer(a.imageProvider, ui.GridViewerCallbacks{})
	a.mainWindow = ui.NewMainWindow(fyneApp, a.actionPanel, a.fileBrowser, a.viewer, a.gridViewer, ui.NewNotifier())
	// The corner ticks for as long as a run is listed, and under the test driver
	// the tick draws where it stands: a test that walks away from a generation it
	// left hanging would keep relabelling a plate next to the test after it.
	t.Cleanup(a.tagRuns.stopAll)
	return a
}

func testPhoto(t *testing.T, withRAW bool) model.Photo {
	t.Helper()
	photo := testPhotoNamed(t, "DSC001.JPG")
	if withRAW {
		photo.RAWPath = strings.TrimSuffix(photo.ImagePath, ".JPG") + ".ARW"
		require.NoError(t, os.WriteFile(photo.RAWPath, []byte("not really a raw"), 0o600))
	}
	return photo
}

func testPhotoNamed(t *testing.T, name string) model.Photo {
	t.Helper()
	photo := model.Photo{Name: name, ImagePath: filepath.Join(t.TempDir(), name)}
	require.NoError(t, os.WriteFile(photo.ImagePath, []byte("not really a jpeg"), 0o600))
	return photo
}

// The report a run makes on its way out touches the corner, and under the test
// driver it does so on the run's own goroutine: a subtest that walks away while
// that is going leaves it drawing text next to the one after it.
func settleRuns(t *testing.T, a *Application, paths ...string) {
	t.Helper()
	require.Eventually(t, func() bool {
		return !slices.ContainsFunc(paths, a.tagRuns.pending)
	}, time.Second, 5*time.Millisecond, "a run outlived the test that started it")
}

func runningNames(a *Application) []string {
	listed := a.tagRuns.live()
	names := make([]string, 0, len(listed))
	for _, item := range listed {
		names = append(names, item.Name)
	}
	return names
}

func generatedTags() model.Tags {
	return model.Tags{Title: "A calm morning by the lake.", Keywords: []string{"lake", "morning"}}
}

// openTestTagsDialog builds the session handleTags would build, without the
// navigator and the shortcuts around it.
func (a *Application) openTestTagsDialog(t *testing.T, photo model.Photo) *tagsSession {
	t.Helper()
	taken, _ := a.imageProvider.PeekStockDate(photo.ImagePath)
	session := &tagsSession{app: a, photo: photo, taken: taken}
	session.dialog = ui.NewTagsDialog(ui.TagsDialogOptions{Filename: photo.Name}, a.mainWindow.Window(),
		ui.TagsDialogCallbacks{})
	a.dialogs.openSelfClosing(dialogTags, session.dialog, session.escape)
	return session
}

func typedTags() model.Tags {
	return model.Tags{Title: "A tram climbs the hill.", Keywords: []string{"tram", "hill"}}
}

// The dialog's entries belong to the ui package, so what the user types is put
// there the same way a file read puts it: into the very fields the caret sits
// in, which is what the dialog hands over when it closes.
func typeIntoDialog(session *tagsSession, typed model.Tags) {
	session.dialog.SetPhotoInfo(typed, time.Time{})
}

func sidecarTags(t *testing.T, photo model.Photo) model.Tags {
	t.Helper()
	path := photo.SidecarPath()
	require.Eventually(t, func() bool {
		written, err := imaging.ReadSidecar(path)
		return err == nil && !written.IsEmpty()
	}, time.Second, 5*time.Millisecond, "the sidecar was never written")
	written, err := imaging.ReadSidecar(path)
	require.NoError(t, err)
	return written
}

func TestTagRunner(t *testing.T) {
	t.Run("a run that keeps its dialog reports to it", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, false)
		session := a.openTestTagsDialog(t, photo)

		a.tagRuns.start(session, tags.Request{Photo: photo})
		<-held.started
		info := held.answerWithTags(t, a, photo.ImagePath)

		assert.Equal(t, generatedTags(), info.Tags)
		assert.Equal(t, generatedTags(), session.dialog.Tags())
	})

	t.Run("a dialog closing over a run leaves the sidecar to it", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, true)
		session := a.openTestTagsDialog(t, photo)

		a.tagRuns.start(session, tags.Request{Photo: photo})
		<-held.started
		typeIntoDialog(session, typedTags())
		session.background()
		require.Equal(t, typedTags(), a.tagRuns.runs[photo.ImagePath].typed,
			"the dialog handed its fields over instead of writing them itself")

		held.answerWithTags(t, a, photo.ImagePath)

		assert.Equal(t, generatedTags(), sidecarTags(t, photo),
			"the run writes what it generated, whatever the dialog held when it closed")
	})

	t.Run("a run that brought nothing writes what the dialog handed over", func(t *testing.T) {
		held := newHeldTagger(model.Tags{}, errors.New("claude fell over"))
		a := newTestApplication(t, held)
		photo := testPhoto(t, true)
		session := a.openTestTagsDialog(t, photo)

		a.tagRuns.start(session, tags.Request{Photo: photo})
		<-held.started
		typeIntoDialog(session, typedTags())
		session.background()

		close(held.release)

		assert.Equal(t, typedTags(), sidecarTags(t, photo),
			"a failed run still owes the sidecar the fields it took over")
	})

	t.Run("a run still going when the app closes writes what it was handed", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, true)
		session := a.openTestTagsDialog(t, photo)

		a.tagRuns.start(session, tags.Request{Photo: photo})
		<-held.started
		typeIntoDialog(session, typedTags())
		session.background()

		a.tagRuns.stopAll()

		assert.Equal(t, typedTags(), sidecarTags(t, photo),
			"the run it was handed to never lands, so the exit is the last chance to write it")
	})

	t.Run("the exit flush writes the sidecar of a photo with no RAW pair", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, false)
		session := a.openTestTagsDialog(t, photo)

		a.tagRuns.start(session, tags.Request{Photo: photo})
		<-held.started
		typeIntoDialog(session, typedTags())
		session.background()

		a.tagRuns.stopAll()

		assert.Equal(t, typedTags(), sidecarTags(t, photo),
			"a JPEG alone owes its own sidecar the fields the dialog handed over")
	})

	t.Run("a reopened dialog takes back the fields it handed over", func(t *testing.T) {
		held := newHeldTagger(model.Tags{}, errors.New("claude fell over"))
		a := newTestApplication(t, held)
		photo := testPhoto(t, true)
		first := a.openTestTagsDialog(t, photo)

		a.tagRuns.start(first, tags.Request{Photo: photo})
		<-held.started
		typeIntoDialog(first, typedTags())
		first.background()

		second := a.openTestTagsDialog(t, photo)
		handed, ok := a.tagRuns.attach(second)
		require.True(t, ok)
		second.dialog.RestoreTags(handed)

		assert.Equal(t, typedTags(), second.dialog.Tags(),
			"a run with a dialog again writes nothing, so the fields belong on screen")

		// Stopped rather than answered: a run that lands fills the dialog from
		// its own goroutine, which under the test driver is a second painter
		// for widgets the next case is already using.
		second.stopRun()
		close(held.release)
		a.tagRuns.running.Wait()
	})

	t.Run("a stopped run leaves its tags to the dialog", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, true)
		session := a.openTestTagsDialog(t, photo)

		a.tagRuns.start(session, tags.Request{Photo: photo})
		<-held.started
		session.stopRun()
		typeIntoDialog(session, typedTags())
		session.close()

		assert.Equal(t, typedTags(), sidecarTags(t, photo),
			"a run that writes nothing must not take the save away from the dialog")
		close(held.release)
	})

	t.Run("a backgrounded run keeps going and saves what it found", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, true)
		session := a.openTestTagsDialog(t, photo)

		a.tagRuns.start(session, tags.Request{Photo: photo})
		<-held.started
		session.background()
		require.Nil(t, a.tagRuns.runs[photo.ImagePath].session)

		info := held.answerWithTags(t, a, photo.ImagePath)

		assert.Equal(t, generatedTags(), info.Tags)
		sidecar := photo.SidecarPath()
		require.Eventually(t, func() bool {
			_, err := os.Stat(sidecar)
			return err == nil
		}, time.Second, 5*time.Millisecond, "the sidecar was never written")
		written, err := imaging.ReadSidecar(sidecar)
		require.NoError(t, err)
		assert.Equal(t, generatedTags(), written)
	})

	t.Run("a backgrounded run writes the sidecar of a photo with no RAW pair", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, false)
		session := a.openTestTagsDialog(t, photo)

		a.tagRuns.start(session, tags.Request{Photo: photo})
		<-held.started
		session.background()

		held.answerWithTags(t, a, photo.ImagePath)

		assert.Equal(t, generatedTags(), sidecarTags(t, photo),
			"the sidecar named after the JPEG is written like the one named after a RAW")
	})

	t.Run("a backgrounded run stays pending until its sidecar is on disk", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, true)
		session := a.openTestTagsDialog(t, photo)

		a.tagRuns.start(session, tags.Request{Photo: photo})
		<-held.started
		session.background()
		require.True(t, a.tagRuns.pending(photo.ImagePath))

		close(held.release)
		require.Eventually(t, func() bool {
			return !a.tagRuns.pending(photo.ImagePath)
		}, time.Second, 5*time.Millisecond, "the run never let go of the photo")

		// The run is only let go of once the file it owes is written, so a copy
		// freed by it finds the sidecar without waiting for anything else.
		require.FileExists(t, photo.SidecarPath())
	})

	t.Run("a sidecar that could not be written leaves the cache empty", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, true)
		// A directory where the sidecar belongs is the one way to make the write
		// fail that says nothing about how the write itself is done.
		require.NoError(t, os.Mkdir(photo.SidecarPath(), 0o700))
		session := a.openTestTagsDialog(t, photo)

		a.tagRuns.start(session, tags.Request{Photo: photo})
		<-held.started
		session.background()

		close(held.release)
		a.tagRuns.running.Wait()
		assert.Never(t, func() bool {
			_, ok := a.imageProvider.PeekStockInfo(photo.ImagePath)
			return ok
		}, 200*time.Millisecond, 5*time.Millisecond, "tags no file holds must not pass for saved")
	})

	t.Run("a cancelled run writes nothing, even when it answers anyway", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, true)
		session := a.openTestTagsDialog(t, photo)

		a.tagRuns.start(session, tags.Request{Photo: photo})
		<-held.started
		session.stopRun()

		// Asked the way a reopened dialog asks it, which is also the only way
		// to read the registry while the run goroutine may still be in it.
		attached, ok := a.tagRuns.attach(session)
		assert.False(t, ok, "a dialog reopened now must not wait on a dead run")
		assert.True(t, attached.IsEmpty())

		held.answerWithNothing(t, a, a.tagRuns, photo.ImagePath)

		assert.NoFileExists(t, photo.SidecarPath())
	})

	t.Run("a reopened dialog attaches to the run it left going", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, false)
		first := a.openTestTagsDialog(t, photo)

		a.tagRuns.start(first, tags.Request{Photo: photo})
		<-held.started
		first.background()

		second := a.openTestTagsDialog(t, photo)
		_, ok := a.tagRuns.attach(second)
		require.True(t, ok)
		assert.Same(t, second, a.tagRuns.runs[photo.ImagePath].session)

		held.answerWithTags(t, a, photo.ImagePath)

		assert.Equal(t, generatedTags(), second.dialog.Tags())
		assert.Empty(t, first.dialog.Tags().Title, "the dialog that let the run go keeps nothing")
	})

	t.Run("waiting on a photo no run touches ends at once", func(t *testing.T) {
		a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))

		a.tagRuns.wait(context.Background(), testPhoto(t, false).ImagePath)
	})

	t.Run("a wait ends when the run lets go of the photo", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, true)
		session := a.openTestTagsDialog(t, photo)

		a.tagRuns.start(session, tags.Request{Photo: photo})
		<-held.started
		session.background()

		waited := make(chan struct{})
		go func() {
			defer close(waited)
			a.tagRuns.wait(context.Background(), photo.ImagePath)
		}()
		select {
		case <-waited:
			t.Fatal("the wait ended while the run was still going")
		case <-time.After(20 * time.Millisecond):
		}

		close(held.release)
		select {
		case <-waited:
		case <-time.After(time.Second):
			t.Fatal("the wait outlived the run")
		}
		assert.FileExists(t, photo.SidecarPath(), "the wait ended before the sidecar was written")
	})

	t.Run("a cancelled context ends the wait and leaves the run alone", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, false)
		session := a.openTestTagsDialog(t, photo)

		a.tagRuns.start(session, tags.Request{Photo: photo})
		<-held.started
		session.background()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		a.tagRuns.wait(ctx, photo.ImagePath)

		assert.True(t, a.tagRuns.pending(photo.ImagePath), "the run must keep going without its waiter")
		held.answerWithTags(t, a, photo.ImagePath)
	})

	t.Run("attaches to nothing when no run is going", func(t *testing.T) {
		a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))

		_, ok := a.tagRuns.attach(a.openTestTagsDialog(t, testPhoto(t, false)))
		assert.False(t, ok)
	})

	t.Run("a date read while the run went beats the one it started with", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, false)
		session := a.openTestTagsDialog(t, photo)
		require.True(t, session.taken.IsZero(), "the cache knows no date when the dialog opens")

		a.tagRuns.start(session, tags.Request{Photo: photo})
		<-held.started
		taken := time.Date(2026, time.June, 13, 10, 0, 0, 0, time.UTC)
		a.imageProvider.StoreStockInfo(photo.ImagePath, imaging.StockInfo{Taken: taken})
		session.background()

		close(held.release)
		var info imaging.StockInfo
		require.Eventually(t, func() bool {
			info, _ = a.imageProvider.PeekStockInfo(photo.ImagePath)
			return !info.Tags.IsEmpty()
		}, time.Second, 5*time.Millisecond, "the run stored nothing")

		assert.Equal(t, taken, info.Taken)
		assert.Equal(t, generatedTags(), info.Tags)
	})

	t.Run("Escape over a running generation stops it and keeps the dialog", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, true)
		session := a.openTestTagsDialog(t, photo)

		a.tagRuns.start(session, tags.Request{Photo: photo})
		<-held.started
		session.dialog.Generating()
		a.handleCancel()

		assert.False(t, session.dialog.IsGenerating(), "the dialog must come back from the run it stopped")
		assert.True(t, a.dialogs.isCurrent(session.dialog), "everything typed into the dialog stays on screen")

		held.answerWithNothing(t, a, a.tagRuns, photo.ImagePath)
		assert.NoFileExists(t, photo.SidecarPath())
	})

	t.Run("Escape over an idle dialog closes it", func(t *testing.T) {
		a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))
		session := a.openTestTagsDialog(t, testPhoto(t, false))

		a.handleCancel()

		assert.False(t, a.dialogs.anyOpen(), "a dialog with no run left to stop closes")
		assert.False(t, a.dialogs.isCurrent(session.dialog))
	})
}

func TestTagRunnerCorner(t *testing.T) {
	t.Run("a run that started is listed", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, false)
		session := a.openTestTagsDialog(t, photo)

		a.tagRuns.start(session, tags.Request{Photo: photo})
		<-held.started

		listed := a.tagRuns.live()
		require.Len(t, listed, 1)
		assert.Equal(t, photo.Name, listed[0].Name)
		assert.False(t, listed[0].Since.IsZero(), "the corner counts from the moment the run started")

		held.answerWithTags(t, a, photo.ImagePath)
		settleRuns(t, a, photo.ImagePath)
	})

	t.Run("a run that landed is gone from the list", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, false)
		session := a.openTestTagsDialog(t, photo)

		a.tagRuns.start(session, tags.Request{Photo: photo})
		<-held.started
		held.answerWithTags(t, a, photo.ImagePath)

		assert.Empty(t, a.tagRuns.live(), "a generation that answered is only writing its file")
		settleRuns(t, a, photo.ImagePath)
	})

	t.Run("a cancelled run is gone from the list before it lets go", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, true)
		session := a.openTestTagsDialog(t, photo)

		a.tagRuns.start(session, tags.Request{Photo: photo})
		<-held.started
		session.stopRun()

		assert.Empty(t, a.tagRuns.live(), "a cancelled run is off the corner before it lets go of its files")

		held.answerWithNothing(t, a, a.tagRuns, photo.ImagePath)
		settleRuns(t, a, photo.ImagePath)
	})

	t.Run("the runs are listed oldest first", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		first := testPhotoNamed(t, "DSC001.JPG")
		second := testPhotoNamed(t, "DSC002.JPG")

		a.tagRuns.start(a.openTestTagsDialog(t, first), tags.Request{Photo: first})
		<-held.started
		a.tagRuns.start(a.openTestTagsDialog(t, second), tags.Request{Photo: second})
		<-held.started

		assert.Equal(t, []string{first.Name, second.Name}, runningNames(a))

		close(held.release)
		settleRuns(t, a, first.ImagePath, second.ImagePath)
	})
}
