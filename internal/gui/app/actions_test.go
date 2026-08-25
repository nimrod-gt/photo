package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"photo/internal/core/library"
	"photo/internal/core/model"
	"photo/internal/core/tags"
)

// backgroundRun leaves a generation going for the photo with no dialog behind
// it, which is the state a copy or a delete has to deal with.
func backgroundRun(t *testing.T, a *Application, held *heldTagger, photo model.Photo) {
	t.Helper()
	session := a.openTestTagsDialog(t, photo)
	a.tagRuns.start(session, tags.Request{Photo: photo})
	<-held.started
	session.background()
}

func TestCopyPhotoFiles(t *testing.T) {
	t.Run("waits for a running generation and copies the sidecar it wrote", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, true)
		backgroundRun(t, a, held, photo)

		dest := t.TempDir()
		copied := make(chan error, 1)
		go func() {
			copied <- a.copyPhotoFiles(context.Background(), photo, dest, library.CopyWithRAW)
		}()
		select {
		case err := <-copied:
			t.Fatal("the copy did not wait for the run:", err)
		case <-time.After(20 * time.Millisecond):
		}

		close(held.release)
		select {
		case err := <-copied:
			require.NoError(t, err)
		case <-time.After(time.Second):
			t.Fatal("the copy outlived the run")
		}

		sidecar := filepath.Base(model.SidecarPath(photo.RAWPath))
		assert.FileExists(t, filepath.Join(dest, sidecar), "the copy left the tags behind")
	})

	t.Run("takes the JPEG alone without waiting for the sidecar", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, true)
		backgroundRun(t, a, held, photo)

		dest := t.TempDir()
		require.NoError(t, a.copyPhotoFiles(context.Background(), photo, dest, library.CopyJPEGOnly))

		assert.FileExists(t, filepath.Join(dest, filepath.Base(photo.ImagePath)))
		close(held.release)
	})

	t.Run("copies a photo no run touches without waiting", func(t *testing.T) {
		a := newTestApplication(t, newHeldTagger(model.Tags{}, nil))
		photo := testPhoto(t, true)
		dest := t.TempDir()

		require.NoError(t, a.copyPhotoFiles(context.Background(), photo, dest, library.CopyWithRAW))

		assert.FileExists(t, filepath.Join(dest, filepath.Base(photo.ImagePath)))
		assert.FileExists(t, filepath.Join(dest, filepath.Base(photo.RAWPath)))
	})
}

func TestDeletePhotoFiles(t *testing.T) {
	t.Run("kills a running generation and leaves no sidecar behind", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, true)
		backgroundRun(t, a, held, photo)

		deleted := make(chan error, 1)
		go func() { deleted <- a.deletePhotoFiles(photo, true) }()
		select {
		case err := <-deleted:
			require.NoError(t, err)
		case <-time.After(time.Second):
			t.Fatal("the delete waited out a run it had killed")
		}

		assert.NoFileExists(t, photo.ImagePath)
		assert.NoFileExists(t, photo.RAWPath)
		// The generation answers anyway once released; a killed run writing its
		// sidecar afterwards is the orphan this whole wait exists to prevent.
		close(held.release)
		sidecar := model.SidecarPath(photo.RAWPath)
		assert.Never(t, func() bool {
			_, err := os.Stat(sidecar)
			return err == nil
		}, 200*time.Millisecond, 5*time.Millisecond, "a killed run wrote a sidecar for a deleted RAW")
	})

	t.Run("a run that wrote its sidecar first leaves nothing cached for the deleted photo", func(t *testing.T) {
		held := newHeldTagger(generatedTags(), nil)
		a := newTestApplication(t, held)
		photo := testPhoto(t, true)
		backgroundRun(t, a, held, photo)

		// The write has to be the one that finishes first, so the delete is
		// held until the sidecar is on disk and only cancels afterwards.
		close(held.release)
		require.Eventually(t, func() bool {
			return !a.tagRuns.pending(photo.ImagePath)
		}, time.Second, 5*time.Millisecond, "the run never let go of the photo")

		require.NoError(t, a.deletePhotoFiles(photo, true))

		assert.Never(t, func() bool {
			_, ok := a.imageProvider.PeekStockInfo(photo.ImagePath)
			return ok
		}, 200*time.Millisecond, 5*time.Millisecond, "tags of a deleted photo stayed in the cache")
	})
}
