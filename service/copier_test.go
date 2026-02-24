package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"photo/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopier_Copy(t *testing.T) {
	fixedTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		hasRAW     bool
		includeRAW bool
		wantRAW    bool
	}{
		{
			name:       "JPEG only",
			hasRAW:     false,
			includeRAW: false,
			wantRAW:    false,
		},
		{
			name:       "JPEG+RAW with includeRAW on",
			hasRAW:     true,
			includeRAW: true,
			wantRAW:    true,
		},
		{
			name:       "JPEG+RAW with includeRAW off",
			hasRAW:     true,
			includeRAW: false,
			wantRAW:    false,
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
			require.NoError(t, c.Copy(photo, destDir, tt.includeRAW))

			destJPEG := filepath.Join(destDir, "photo.jpg")
			data, err := os.ReadFile(destJPEG)
			require.NoError(t, err)
			assert.Equal(t, jpegContent, data)

			info, err := os.Stat(destJPEG)
			require.NoError(t, err)
			assert.Equal(t, fixedTime, info.ModTime().UTC())

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

	t.Run("missing destination directory", func(t *testing.T) {
		srcDir := t.TempDir()
		jpegPath := filepath.Join(srcDir, "photo.jpg")
		require.NoError(t, os.WriteFile(jpegPath, nil, 0600))

		photo := model.Photo{ImagePath: jpegPath, Name: "photo.jpg"}
		c := NewCopier()
		err := c.Copy(photo, "/nonexistent/destination", false)
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
		err := c.Copy(photo, destFile, false)
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
		require.NoError(t, c.Copy(photo, destDir, false))

		data, err := os.ReadFile(existingPath)
		require.NoError(t, err)
		assert.Equal(t, jpegContent, data)
	})
}
