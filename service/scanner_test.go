package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanner_ScanDirectory(t *testing.T) {
	scanner := NewScanner()

	t.Run("JPEG only", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "a.jpg")
		touch(t, dir, "b.txt")
		touch(t, dir, "c.png")

		photos, err := scanner.ScanDirectory(dir)
		require.NoError(t, err)
		assert.Len(t, photos, 1)
		assert.Equal(t, "a.jpg", photos[0].Name)
	})

	t.Run("case-insensitive extensions", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "a.JPG")
		touch(t, dir, "b.JPEG")
		touch(t, dir, "c.Jpg")

		photos, err := scanner.ScanDirectory(dir)
		require.NoError(t, err)
		assert.Len(t, photos, 3)
	})

	t.Run("skips directories", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "a.jpg")
		require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir.jpg"), 0755))

		photos, err := scanner.ScanDirectory(dir)
		require.NoError(t, err)
		assert.Len(t, photos, 1)
	})

	t.Run("empty directory", func(t *testing.T) {
		dir := t.TempDir()
		photos, err := scanner.ScanDirectory(dir)
		require.NoError(t, err)
		assert.Empty(t, photos)
	})

	t.Run("non-existent directory", func(t *testing.T) {
		_, err := scanner.ScanDirectory("/nonexistent/path")
		assert.Error(t, err)
	})

	t.Run("with RAW pair", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "photo.jpg")
		touch(t, dir, "photo.ARW")

		photos, err := scanner.ScanDirectory(dir)
		require.NoError(t, err)
		require.Len(t, photos, 1)
		assert.Equal(t, "photo.jpg", photos[0].Name)
		assert.True(t, photos[0].HasRAW())
	})

	t.Run("with lowercase RAW pair", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "photo.jpg")
		touch(t, dir, "photo.arw")

		photos, err := scanner.ScanDirectory(dir)
		require.NoError(t, err)
		require.Len(t, photos, 1)
		assert.True(t, photos[0].HasRAW())
	})
}

func TestScanner_SortPhotos(t *testing.T) {
	scanner := NewScanner()

	t.Run("by name", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "c.jpg")
		touch(t, dir, "a.jpg")
		touch(t, dir, "b.jpg")

		photos, err := scanner.ScanDirectory(dir)
		require.NoError(t, err)

		scanner.SortPhotos(photos, SortByName)
		assert.Equal(t, "a.jpg", photos[0].Name)
		assert.Equal(t, "b.jpg", photos[1].Name)
		assert.Equal(t, "c.jpg", photos[2].Name)
	})

	t.Run("by time", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "old.jpg")
		touch(t, dir, "new.jpg")

		now := time.Now()
		require.NoError(t, os.Chtimes(filepath.Join(dir, "old.jpg"), now.Add(-2*time.Hour), now.Add(-2*time.Hour)))
		require.NoError(t, os.Chtimes(filepath.Join(dir, "new.jpg"), now, now))

		photos, err := scanner.ScanDirectory(dir)
		require.NoError(t, err)

		scanner.SortPhotos(photos, SortByTime)
		assert.Equal(t, "old.jpg", photos[0].Name)
		assert.Equal(t, "new.jpg", photos[1].Name)
	})
}

func TestScanner_ListDirectories(t *testing.T) {
	scanner := NewScanner()

	t.Run("basic", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, "subA"), 0755))
		require.NoError(t, os.Mkdir(filepath.Join(dir, "subB"), 0755))
		touch(t, dir, "file.txt")

		dirs, err := scanner.ListDirectories(dir)
		require.NoError(t, err)
		assert.Len(t, dirs, 2)
	})

	t.Run("excludes hidden", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, ".hidden"), 0755))
		require.NoError(t, os.Mkdir(filepath.Join(dir, "visible"), 0755))

		dirs, err := scanner.ListDirectories(dir)
		require.NoError(t, err)
		assert.Len(t, dirs, 1)
		assert.Contains(t, dirs[0], "visible")
	})
}

func touch(t *testing.T, dir, name string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), nil, 0600))
}
