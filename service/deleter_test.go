package service

import (
	"os"
	"path/filepath"
	"testing"

	"photo/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleter_Delete(t *testing.T) {
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
