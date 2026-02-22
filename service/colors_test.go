package service

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"photo/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPhoto(dir, name string) model.Photo {
	return model.Photo{JPEGPath: filepath.Join(dir, name), Name: name}
}

func TestColorService_GetColors(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		dir := t.TempDir()
		svc := NewColorService()
		colors, err := svc.GetColors(testPhoto(dir, "photo.jpg"))
		require.NoError(t, err)
		assert.Empty(t, colors)
	})
}

func TestColorService_ToggleColor(t *testing.T) {
	t.Run("add and get", func(t *testing.T) {
		dir := t.TempDir()
		svc := NewColorService()
		photo := testPhoto(dir, "photo.jpg")

		require.NoError(t, svc.ToggleColor(photo, model.ColorRed))
		colors, err := svc.GetColors(photo)
		require.NoError(t, err)
		assert.Contains(t, colors, model.ColorRed)
	})

	t.Run("toggle remove and get", func(t *testing.T) {
		dir := t.TempDir()
		svc := NewColorService()
		photo := testPhoto(dir, "photo.jpg")

		require.NoError(t, svc.ToggleColor(photo, model.ColorRed))
		require.NoError(t, svc.ToggleColor(photo, model.ColorRed))
		colors, err := svc.GetColors(photo)
		require.NoError(t, err)
		assert.NotContains(t, colors, model.ColorRed)
	})
}

func TestColorService_HasColor(t *testing.T) {
	dir := t.TempDir()
	svc := NewColorService()
	photo := testPhoto(dir, "photo.jpg")

	has, err := svc.HasColor(photo, model.ColorGreen)
	require.NoError(t, err)
	assert.False(t, has)

	require.NoError(t, svc.ToggleColor(photo, model.ColorGreen))

	has, err = svc.HasColor(photo, model.ColorGreen)
	require.NoError(t, err)
	assert.True(t, has)
}

func TestColorService_RemoveColors(t *testing.T) {
	t.Run("existing", func(t *testing.T) {
		dir := t.TempDir()
		svc := NewColorService()
		photo := testPhoto(dir, "photo.jpg")

		require.NoError(t, svc.ToggleColor(photo, model.ColorRed))
		require.NoError(t, svc.RemoveColors(photo))

		colors, err := svc.GetColors(photo)
		require.NoError(t, err)
		assert.Empty(t, colors)
	})

	t.Run("non-existing", func(t *testing.T) {
		dir := t.TempDir()
		svc := NewColorService()
		assert.NoError(t, svc.RemoveColors(testPhoto(dir, "photo.jpg")))
	})
}

func TestColorService_InvalidateCache(t *testing.T) {
	dir := t.TempDir()
	svc := NewColorService()
	photo := testPhoto(dir, "photo.jpg")

	require.NoError(t, svc.ToggleColor(photo, model.ColorRed))

	cm := model.ColorMap{"photo.jpg": {model.ColorBlue}}
	require.NoError(t, model.SaveColors(dir, cm))

	svc.InvalidateCache(dir)

	colors, err := svc.GetColors(photo)
	require.NoError(t, err)
	assert.Contains(t, colors, model.ColorBlue)
	assert.NotContains(t, colors, model.ColorRed)
}

func TestColorService_ConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	svc := NewColorService()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			_ = svc.ToggleColor(testPhoto(dir, "photo.jpg"), model.ColorRed)
		}()
		go func() {
			defer wg.Done()
			_, _ = svc.GetColors(testPhoto(dir, "photo.jpg"))
		}()
		go func() {
			defer wg.Done()
			_, _ = svc.HasColor(testPhoto(dir, "photo.jpg"), model.ColorRed)
		}()
	}
	wg.Wait()
}

func TestColorService_InvalidateCache_ReReadsFromDisk(t *testing.T) {
	dir := t.TempDir()
	svc := NewColorService()
	photo := testPhoto(dir, "photo.jpg")

	require.NoError(t, svc.ToggleColor(photo, model.ColorGreen))

	data := `{"photo.jpg":["red"]}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, model.ColorsFileName), []byte(data), 0600))

	colors, err := svc.GetColors(photo)
	require.NoError(t, err)
	assert.Contains(t, colors, model.ColorGreen)

	svc.InvalidateCache(dir)

	colors, err = svc.GetColors(photo)
	require.NoError(t, err)
	assert.Contains(t, colors, model.ColorRed)
	assert.NotContains(t, colors, model.ColorGreen)
}
