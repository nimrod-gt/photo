package library

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"photo/internal/core/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPhoto(dir, name string) model.Photo {
	return model.Photo{ImagePath: filepath.Join(dir, name), Name: name}
}

func TestColorService_GetColors(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		dir := t.TempDir()
		svc := NewColorService()
		colors, err := svc.GetColors(testPhoto(dir, "photo.jpg"))
		require.NoError(t, err)
		assert.Empty(t, colors)
	})
}

func TestColorService_ToggleColor(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

func TestColorService_RemoveMultipleColors(t *testing.T) {
	t.Parallel()

	t.Run("removes colors for multiple photos in same dir", func(t *testing.T) {
		dir := t.TempDir()
		svc := NewColorService()
		photo1 := testPhoto(dir, "a.jpg")
		photo2 := testPhoto(dir, "b.jpg")
		photo3 := testPhoto(dir, "c.jpg")

		require.NoError(t, svc.ToggleColor(photo1, model.ColorRed))
		require.NoError(t, svc.ToggleColor(photo2, model.ColorGreen))
		require.NoError(t, svc.ToggleColor(photo3, model.ColorBlue))

		require.NoError(t, svc.RemoveMultipleColors([]model.Photo{photo1, photo2}))

		colors, err := svc.GetColors(photo1)
		require.NoError(t, err)
		assert.Empty(t, colors)

		colors, err = svc.GetColors(photo2)
		require.NoError(t, err)
		assert.Empty(t, colors)

		colors, err = svc.GetColors(photo3)
		require.NoError(t, err)
		assert.Contains(t, colors, model.ColorBlue)
	})

	t.Run("handles photos across multiple dirs", func(t *testing.T) {
		dir1 := t.TempDir()
		dir2 := t.TempDir()
		svc := NewColorService()
		photo1 := testPhoto(dir1, "a.jpg")
		photo2 := testPhoto(dir2, "b.jpg")

		require.NoError(t, svc.ToggleColor(photo1, model.ColorRed))
		require.NoError(t, svc.ToggleColor(photo2, model.ColorGreen))

		require.NoError(t, svc.RemoveMultipleColors([]model.Photo{photo1, photo2}))

		colors, err := svc.GetColors(photo1)
		require.NoError(t, err)
		assert.Empty(t, colors)

		colors, err = svc.GetColors(photo2)
		require.NoError(t, err)
		assert.Empty(t, colors)
	})

	t.Run("empty list is no-op", func(t *testing.T) {
		svc := NewColorService()
		assert.NoError(t, svc.RemoveMultipleColors(nil))
	})
}

func TestColorService_RemoveColorLabels(t *testing.T) {
	t.Parallel()

	t.Run("removes only specified color keeping others", func(t *testing.T) {
		dir := t.TempDir()
		svc := NewColorService()
		photo := testPhoto(dir, "photo.jpg")

		require.NoError(t, svc.ToggleColor(photo, model.ColorRed))
		require.NoError(t, svc.ToggleColor(photo, model.ColorGreen))

		require.NoError(t, svc.RemoveColorLabels([]model.Photo{photo}, []model.ColorLabel{model.ColorRed}))

		colors, err := svc.GetColors(photo)
		require.NoError(t, err)
		assert.NotContains(t, colors, model.ColorRed)
		assert.Contains(t, colors, model.ColorGreen)
	})

	t.Run("removes key when last color stripped", func(t *testing.T) {
		dir := t.TempDir()
		svc := NewColorService()
		photo := testPhoto(dir, "photo.jpg")

		require.NoError(t, svc.ToggleColor(photo, model.ColorRed))
		require.NoError(t, svc.RemoveColorLabels([]model.Photo{photo}, []model.ColorLabel{model.ColorRed}))

		cm, err := model.LoadColors(dir)
		require.NoError(t, err)
		_, exists := cm["photo.jpg"]
		assert.False(t, exists)
	})

	t.Run("persists across dirs for fresh service", func(t *testing.T) {
		dir1 := t.TempDir()
		dir2 := t.TempDir()
		svc := NewColorService()
		photo1 := testPhoto(dir1, "a.jpg")
		photo2 := testPhoto(dir2, "b.jpg")

		require.NoError(t, svc.ToggleColor(photo1, model.ColorRed))
		require.NoError(t, svc.ToggleColor(photo1, model.ColorBlue))
		require.NoError(t, svc.ToggleColor(photo2, model.ColorRed))

		require.NoError(t, svc.RemoveColorLabels([]model.Photo{photo1, photo2}, []model.ColorLabel{model.ColorRed}))

		fresh := NewColorService()
		colors, err := fresh.GetColors(photo1)
		require.NoError(t, err)
		assert.NotContains(t, colors, model.ColorRed)
		assert.Contains(t, colors, model.ColorBlue)

		colors, err = fresh.GetColors(photo2)
		require.NoError(t, err)
		assert.Empty(t, colors)
	})

	t.Run("untouched photo in same dir keeps colors", func(t *testing.T) {
		dir := t.TempDir()
		svc := NewColorService()
		photo1 := testPhoto(dir, "a.jpg")
		photo2 := testPhoto(dir, "b.jpg")

		require.NoError(t, svc.ToggleColor(photo1, model.ColorRed))
		require.NoError(t, svc.ToggleColor(photo2, model.ColorRed))

		require.NoError(t, svc.RemoveColorLabels([]model.Photo{photo1}, []model.ColorLabel{model.ColorRed}))

		colors, err := svc.GetColors(photo2)
		require.NoError(t, err)
		assert.Contains(t, colors, model.ColorRed)
	})

	t.Run("empty list is no-op", func(t *testing.T) {
		svc := NewColorService()
		assert.NoError(t, svc.RemoveColorLabels(nil, []model.ColorLabel{model.ColorRed}))
	})
}

func TestColorService_InvalidateCache(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	dir := t.TempDir()
	svc := NewColorService()

	var wg sync.WaitGroup
	for range 50 {
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
	t.Parallel()

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
