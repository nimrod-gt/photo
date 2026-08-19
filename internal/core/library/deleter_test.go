package library

import (
	"os"
	"path/filepath"
	"testing"

	"photo/internal/core/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleter_Delete(t *testing.T) {
	t.Parallel()

	t.Run("JPEG only", func(t *testing.T) {
		dir := t.TempDir()
		jpegPath := filepath.Join(dir, "photo.jpg")
		require.NoError(t, os.WriteFile(jpegPath, nil, 0600))

		photo := model.Photo{ImagePath: jpegPath, Name: "photo.jpg"}
		d := NewDeleter()
		require.NoError(t, d.Delete(photo))

		_, err := os.Stat(jpegPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("with RAW pair", func(t *testing.T) {
		dir := t.TempDir()
		jpegPath := filepath.Join(dir, "photo.jpg")
		rawPath := filepath.Join(dir, "photo.ARW")
		require.NoError(t, os.WriteFile(jpegPath, nil, 0600))
		require.NoError(t, os.WriteFile(rawPath, nil, 0600))

		photo := model.Photo{ImagePath: jpegPath, RAWPath: rawPath, Name: "photo.jpg"}
		d := NewDeleter()
		require.NoError(t, d.Delete(photo))

		_, err := os.Stat(jpegPath)
		assert.True(t, os.IsNotExist(err))
		_, err = os.Stat(rawPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("JPEG already gone", func(t *testing.T) {
		photo := model.Photo{ImagePath: "/nonexistent/photo.jpg", Name: "photo.jpg"}
		d := NewDeleter()
		assert.NoError(t, d.Delete(photo))
	})

	t.Run("RAW already gone", func(t *testing.T) {
		dir := t.TempDir()
		jpegPath := filepath.Join(dir, "photo.jpg")
		require.NoError(t, os.WriteFile(jpegPath, nil, 0600))

		photo := model.Photo{ImagePath: jpegPath, RAWPath: "/nonexistent/photo.ARW", Name: "photo.jpg"}
		d := NewDeleter()
		assert.NoError(t, d.Delete(photo))
	})
}

func TestDeleter_DeleteWithOption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		hasRAW     bool
		includeRAW bool
		wantRAW    bool
	}{
		{
			name:       "JPEG only without RAW option",
			hasRAW:     false,
			includeRAW: false,
			wantRAW:    false,
		},
		{
			name:       "JPEG+RAW with includeRAW on",
			hasRAW:     true,
			includeRAW: true,
			wantRAW:    false,
		},
		{
			name:       "JPEG+RAW with includeRAW off keeps RAW",
			hasRAW:     true,
			includeRAW: false,
			wantRAW:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			jpegPath := filepath.Join(dir, "photo.jpg")
			require.NoError(t, os.WriteFile(jpegPath, nil, 0600))

			photo := model.Photo{ImagePath: jpegPath, Name: "photo.jpg"}

			if tt.hasRAW {
				rawPath := filepath.Join(dir, "photo.ARW")
				require.NoError(t, os.WriteFile(rawPath, nil, 0600))
				photo.RAWPath = rawPath
			}

			d := NewDeleter()
			require.NoError(t, d.DeleteWithOption(photo, tt.includeRAW))

			_, err := os.Stat(jpegPath)
			assert.True(t, os.IsNotExist(err), "JPEG should be deleted")

			if tt.hasRAW {
				_, err := os.Stat(photo.RAWPath)
				if tt.wantRAW {
					assert.NoError(t, err, "RAW should still exist")
				} else {
					assert.True(t, os.IsNotExist(err), "RAW should be deleted")
				}
			}
		})
	}
}

func TestDeleter_DeletesTheSidecarWithTheRAW(t *testing.T) {
	t.Parallel()

	t.Run("the sidecar goes with the RAW", func(t *testing.T) {
		t.Parallel()
		photo, sidecar := writePairWithSidecar(t)

		require.NoError(t, NewDeleter().Delete(photo))

		assert.NoFileExists(t, photo.RAWPath)
		assert.NoFileExists(t, sidecar)
	})

	t.Run("the sidecar stays when the RAW is kept", func(t *testing.T) {
		t.Parallel()
		photo, sidecar := writePairWithSidecar(t)

		require.NoError(t, NewDeleter().DeleteWithOption(photo, false))

		assert.FileExists(t, photo.RAWPath)
		assert.FileExists(t, sidecar)
	})

	// The photo is gone by the time the sidecar is reached, so failing here would
	// leave it in the list as an entry with no file behind it.
	t.Run("a sidecar that cannot be removed does not fail the delete", func(t *testing.T) {
		t.Parallel()
		photo, sidecar := writePairWithSidecar(t)
		require.NoError(t, os.Remove(sidecar))
		require.NoError(t, os.Mkdir(sidecar, 0700))
		require.NoError(t, os.WriteFile(filepath.Join(sidecar, "blocker"), nil, 0600))

		require.NoError(t, NewDeleter().Delete(photo))

		assert.NoFileExists(t, photo.ImagePath)
		assert.NoFileExists(t, photo.RAWPath)
		assert.DirExists(t, sidecar)
	})

	t.Run("a RAW without a sidecar deletes cleanly", func(t *testing.T) {
		t.Parallel()
		photo, sidecar := writePairWithSidecar(t)
		require.NoError(t, os.Remove(sidecar))

		assert.NoError(t, NewDeleter().Delete(photo))
	})
}

func writePairWithSidecar(t *testing.T) (model.Photo, string) {
	t.Helper()
	dir := t.TempDir()
	photo := model.Photo{
		ImagePath: filepath.Join(dir, "photo.jpg"),
		RAWPath:   filepath.Join(dir, "photo.ARW"),
		Name:      "photo.jpg",
	}
	sidecar := model.SidecarPath(photo.RAWPath)
	for _, path := range []string{photo.ImagePath, photo.RAWPath, sidecar} {
		require.NoError(t, os.WriteFile(path, nil, 0600))
	}
	return photo, sidecar
}
