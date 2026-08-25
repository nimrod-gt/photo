package app

import (
	"context"
	"os"
	"path/filepath"
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

func (h *heldTagger) answerWithNothing(t *testing.T, a *Application, r *tagRunner, path string) {
	t.Helper()
	close(h.release)
	r.running.Wait()
	assert.Never(t, func() bool {
		_, ok := a.imageProvider.PeekStockInfo(path)
		return ok
	}, 50*time.Millisecond, 5*time.Millisecond, "the run stored tags it was told to drop")
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
	return a
}

func testPhoto(t *testing.T, withRAW bool) model.Photo {
	t.Helper()
	dir := t.TempDir()
	photo := model.Photo{Name: "DSC001.JPG", ImagePath: filepath.Join(dir, "DSC001.JPG")}
	require.NoError(t, os.WriteFile(photo.ImagePath, []byte("not really a jpeg"), 0o600))
	if withRAW {
		photo.RAWPath = filepath.Join(dir, "DSC001.ARW")
		require.NoError(t, os.WriteFile(photo.RAWPath, []byte("not really a raw"), 0o600))
	}
	return photo
}

func generatedTags() model.Tags {
	return model.Tags{Title: "A calm morning by the lake.", Keywords: []string{"lake", "morning"}}
}

// openTestTagsDialog builds the session handleTags would build, without the
// navigator and the shortcuts around it.
func (a *Application) openTestTagsDialog(t *testing.T, photo model.Photo) *tagsSession {
	t.Helper()
	taken, _ := a.imageProvider.PeekStockDate(photo.ImagePath)
	session := &tagsSession{app: a, photo: photo, prefs: a.fyneApp.Preferences(), taken: taken}
	session.dialog = ui.NewTagsDialog(ui.TagsDialogOptions{Filename: photo.Name}, a.mainWindow.Window(),
		ui.TagsDialogCallbacks{})
	a.dialogs.open(dialogTags, session.dialog, session.cancelRun)
	return session
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
		sidecar := model.SidecarPath(photo.RAWPath)
		require.Eventually(t, func() bool {
			_, err := os.Stat(sidecar)
			return err == nil
		}, time.Second, 5*time.Millisecond, "the sidecar was never written")
		written, err := imaging.ReadSidecar(sidecar)
		require.NoError(t, err)
		assert.Equal(t, generatedTags(), written)
	})

	t.Run("a cancelled run writes nothing, even when it answers anyway", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, true)
		session := a.openTestTagsDialog(t, photo)

		a.tagRuns.start(session, tags.Request{Photo: photo})
		<-held.started
		session.cancelRun()

		assert.Empty(t, a.tagRuns.runs, "a cancelled run leaves the registry at once")
		assert.False(t, a.tagRuns.attach(session), "a dialog reopened now must not wait on a dead run")

		held.answerWithNothing(t, a, a.tagRuns, photo.ImagePath)

		assert.NoFileExists(t, model.SidecarPath(photo.RAWPath))
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
		require.True(t, a.tagRuns.attach(second))
		assert.Same(t, second, a.tagRuns.runs[photo.ImagePath].session)

		held.answerWithTags(t, a, photo.ImagePath)

		assert.Equal(t, generatedTags(), second.dialog.Tags())
		assert.Empty(t, first.dialog.Tags().Title, "the dialog that let the run go keeps nothing")
	})

	t.Run("attaches to nothing when no run is going", func(t *testing.T) {
		a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))

		assert.False(t, a.tagRuns.attach(a.openTestTagsDialog(t, testPhoto(t, false))))
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
}
