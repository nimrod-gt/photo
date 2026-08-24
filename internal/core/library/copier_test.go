package library

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"photo/internal/core/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopier_Copy(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		hasRAW  bool
		mode    CopyMode
		wantRAW bool
		wantImg bool
	}{
		{
			name:    "JPEG only mode",
			hasRAW:  false,
			mode:    CopyJPEGOnly,
			wantRAW: false,
			wantImg: true,
		},
		{
			name:    "JPEG+RAW with CopyWithRAW",
			hasRAW:  true,
			mode:    CopyWithRAW,
			wantRAW: true,
			wantImg: true,
		},
		{
			name:    "JPEG+RAW with CopyJPEGOnly",
			hasRAW:  true,
			mode:    CopyJPEGOnly,
			wantRAW: false,
			wantImg: true,
		},
		{
			name:    "CopyOnlyRAW copies only RAW",
			hasRAW:  true,
			mode:    CopyOnlyRAW,
			wantRAW: true,
			wantImg: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srcDir := t.TempDir()
			destDir := t.TempDir()

			jpegContent := []byte("jpeg-data")
			jpegPath := filepath.Join(srcDir, "photo.jpg")
			require.NoError(t, os.WriteFile(jpegPath, jpegContent, 0600))
			require.NoError(t, os.Chtimes(jpegPath, fixedTime, fixedTime))

			photo := model.Photo{ImagePath: jpegPath, Name: "photo.jpg"}

			if tt.hasRAW {
				rawContent := []byte("raw-data")
				rawPath := filepath.Join(srcDir, "photo.ARW")
				require.NoError(t, os.WriteFile(rawPath, rawContent, 0600))
				require.NoError(t, os.Chtimes(rawPath, fixedTime, fixedTime))
				photo.RAWPath = rawPath
			}

			c := NewCopier()
			require.NoError(t, c.Copy(photo, destDir, tt.mode))

			destJPEG := filepath.Join(destDir, "photo.jpg")
			if tt.wantImg {
				data, err := os.ReadFile(destJPEG)
				require.NoError(t, err)
				assert.Equal(t, jpegContent, data)

				info, err := os.Stat(destJPEG)
				require.NoError(t, err)
				assert.Equal(t, fixedTime, info.ModTime().UTC())
			} else {
				_, err := os.Stat(destJPEG)
				assert.True(t, os.IsNotExist(err))
			}

			destRAW := filepath.Join(destDir, "photo.ARW")
			if tt.wantRAW {
				data, err := os.ReadFile(destRAW)
				require.NoError(t, err)
				assert.Equal(t, []byte("raw-data"), data)

				info, err := os.Stat(destRAW)
				require.NoError(t, err)
				assert.Equal(t, fixedTime, info.ModTime().UTC())
			} else {
				_, err := os.Stat(destRAW)
				assert.True(t, os.IsNotExist(err))
			}
		})
	}

	t.Run("CopyOnlyRAW without RAW returns error", func(t *testing.T) {
		srcDir := t.TempDir()
		destDir := t.TempDir()

		jpegPath := filepath.Join(srcDir, "photo.jpg")
		require.NoError(t, os.WriteFile(jpegPath, []byte("jpeg"), 0600))

		photo := model.Photo{ImagePath: jpegPath, Name: "photo.jpg"}
		c := NewCopier()
		err := c.Copy(photo, destDir, CopyOnlyRAW)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no RAW file")
	})

	t.Run("invalid copy mode returns error", func(t *testing.T) {
		srcDir := t.TempDir()
		destDir := t.TempDir()

		jpegPath := filepath.Join(srcDir, "photo.jpg")
		require.NoError(t, os.WriteFile(jpegPath, []byte("jpeg"), 0600))

		photo := model.Photo{ImagePath: jpegPath, Name: "photo.jpg"}
		c := NewCopier()
		err := c.Copy(photo, destDir, CopyMode(99))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown copy mode")
	})

	t.Run("missing destination directory", func(t *testing.T) {
		srcDir := t.TempDir()
		jpegPath := filepath.Join(srcDir, "photo.jpg")
		require.NoError(t, os.WriteFile(jpegPath, nil, 0600))

		photo := model.Photo{ImagePath: jpegPath, Name: "photo.jpg"}
		c := NewCopier()
		err := c.Copy(photo, "/nonexistent/destination", CopyJPEGOnly)
		assert.Error(t, err)
	})

	t.Run("destination is a file not directory", func(t *testing.T) {
		srcDir := t.TempDir()
		jpegPath := filepath.Join(srcDir, "photo.jpg")
		require.NoError(t, os.WriteFile(jpegPath, nil, 0600))

		destFile := filepath.Join(t.TempDir(), "afile")
		require.NoError(t, os.WriteFile(destFile, nil, 0600))

		photo := model.Photo{ImagePath: jpegPath, Name: "photo.jpg"}
		c := NewCopier()
		err := c.Copy(photo, destFile, CopyJPEGOnly)
		assert.Error(t, err)
	})

	t.Run("overwrites existing file at destination", func(t *testing.T) {
		srcDir := t.TempDir()
		destDir := t.TempDir()

		jpegContent := []byte("new-jpeg-data")
		jpegPath := filepath.Join(srcDir, "photo.jpg")
		require.NoError(t, os.WriteFile(jpegPath, jpegContent, 0600))

		existingPath := filepath.Join(destDir, "photo.jpg")
		require.NoError(t, os.WriteFile(existingPath, []byte("old-data"), 0600))

		photo := model.Photo{ImagePath: jpegPath, Name: "photo.jpg"}
		c := NewCopier()
		require.NoError(t, c.Copy(photo, destDir, CopyJPEGOnly))

		data, err := os.ReadFile(existingPath)
		require.NoError(t, err)
		assert.Equal(t, jpegContent, data)
	})
}

func TestCopier_CopyWithContext(t *testing.T) {
	t.Parallel()

	t.Run("cancelled context skips copy", func(t *testing.T) {
		srcDir := t.TempDir()
		destDir := t.TempDir()

		jpegPath := filepath.Join(srcDir, "photo.jpg")
		require.NoError(t, os.WriteFile(jpegPath, []byte("data"), 0600))

		photo := model.Photo{ImagePath: jpegPath, Name: "photo.jpg"}
		c := NewCopier()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := c.CopyWithContext(ctx, photo, destDir, CopyJPEGOnly)
		require.ErrorIs(t, err, context.Canceled)

		_, statErr := os.Stat(filepath.Join(destDir, "photo.jpg"))
		assert.True(t, os.IsNotExist(statErr))
	})

	t.Run("normal context copies files", func(t *testing.T) {
		srcDir := t.TempDir()
		destDir := t.TempDir()

		jpegContent := []byte("jpeg-data")
		jpegPath := filepath.Join(srcDir, "photo.jpg")
		require.NoError(t, os.WriteFile(jpegPath, jpegContent, 0600))

		rawContent := []byte("raw-data")
		rawPath := filepath.Join(srcDir, "photo.ARW")
		require.NoError(t, os.WriteFile(rawPath, rawContent, 0600))

		photo := model.Photo{ImagePath: jpegPath, RAWPath: rawPath, Name: "photo.jpg"}
		c := NewCopier()

		require.NoError(t, c.CopyWithContext(context.Background(), photo, destDir, CopyWithRAW))

		data, err := os.ReadFile(filepath.Join(destDir, "photo.jpg"))
		require.NoError(t, err)
		assert.Equal(t, jpegContent, data)

		data, err = os.ReadFile(filepath.Join(destDir, "photo.ARW"))
		require.NoError(t, err)
		assert.Equal(t, rawContent, data)
	})

	t.Run("cancellation stops a copy between chunks", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		reader := cancellableReader{ctx: ctx, r: strings.NewReader("data")}

		buf := make([]byte, 2)
		n, err := reader.Read(buf)
		require.NoError(t, err)
		assert.Equal(t, 2, n)

		cancel()
		_, err = reader.Read(buf)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("without RAW option", func(t *testing.T) {
		srcDir := t.TempDir()
		destDir := t.TempDir()

		jpegPath := filepath.Join(srcDir, "photo.jpg")
		require.NoError(t, os.WriteFile(jpegPath, []byte("jpeg"), 0600))
		rawPath := filepath.Join(srcDir, "photo.ARW")
		require.NoError(t, os.WriteFile(rawPath, []byte("raw"), 0600))

		photo := model.Photo{ImagePath: jpegPath, RAWPath: rawPath, Name: "photo.jpg"}
		c := NewCopier()

		require.NoError(t, c.CopyWithContext(context.Background(), photo, destDir, CopyJPEGOnly))

		_, err := os.Stat(filepath.Join(destDir, "photo.jpg"))
		require.NoError(t, err)
		_, err = os.Stat(filepath.Join(destDir, "photo.ARW"))
		assert.True(t, os.IsNotExist(err))
	})
}

func TestCopier_CopiesTheSidecarWithTheRAW(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode CopyMode
		want bool
	}{
		{name: "CopyWithRAW takes the sidecar", mode: CopyWithRAW, want: true},
		{name: "CopyOnlyRAW takes the sidecar", mode: CopyOnlyRAW, want: true},
		{name: "CopyJPEGOnly leaves the sidecar", mode: CopyJPEGOnly, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srcDir, destDir := t.TempDir(), t.TempDir()
			photo := writePair(t, srcDir)
			require.NoError(t, os.WriteFile(model.SidecarPath(photo.RAWPath), []byte("<x:xmpmeta/>"), 0600))

			require.NoError(t, NewCopier().Copy(photo, destDir, tt.mode))

			copied := filepath.Join(destDir, "photo.xmp")
			if !tt.want {
				assert.NoFileExists(t, copied)
				return
			}
			content, err := os.ReadFile(copied)
			require.NoError(t, err)
			assert.Equal(t, "<x:xmpmeta/>", string(content))
		})
	}
}

func TestCopier_CopiesARAWWithoutASidecar(t *testing.T) {
	t.Parallel()

	srcDir, destDir := t.TempDir(), t.TempDir()
	photo := writePair(t, srcDir)

	require.NoError(t, NewCopier().Copy(photo, destDir, CopyWithRAW))

	assert.FileExists(t, filepath.Join(destDir, "photo.ARW"))
	assert.NoFileExists(t, filepath.Join(destDir, "photo.xmp"))
}

func writePair(t *testing.T, dir string) model.Photo {
	t.Helper()
	jpegPath := filepath.Join(dir, "photo.jpg")
	rawPath := filepath.Join(dir, "photo.ARW")
	require.NoError(t, os.WriteFile(jpegPath, []byte("jpeg-data"), 0600))
	require.NoError(t, os.WriteFile(rawPath, []byte("raw-data"), 0600))
	return model.Photo{ImagePath: jpegPath, RAWPath: rawPath, Name: "photo.jpg"}
}
