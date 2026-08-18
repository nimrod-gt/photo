package model

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestColorMap_HasColor(t *testing.T) {
	t.Parallel()

	t.Run("empty map", func(t *testing.T) {
		cm := make(ColorMap)
		assert.False(t, cm.HasColor("photo.jpg", ColorRed))
	})

	t.Run("has color", func(t *testing.T) {
		cm := ColorMap{"photo.jpg": {ColorRed, ColorGreen}}
		assert.True(t, cm.HasColor("photo.jpg", ColorRed))
		assert.True(t, cm.HasColor("photo.jpg", ColorGreen))
		assert.False(t, cm.HasColor("photo.jpg", ColorBlue))
	})
}

func TestColorMap_ToggleColor(t *testing.T) {
	t.Parallel()

	t.Run("add color", func(t *testing.T) {
		cm := make(ColorMap)
		cm.ToggleColor("photo.jpg", ColorRed)
		assert.True(t, cm.HasColor("photo.jpg", ColorRed))
	})

	t.Run("remove color", func(t *testing.T) {
		cm := ColorMap{"photo.jpg": {ColorRed}}
		cm.ToggleColor("photo.jpg", ColorRed)
		assert.False(t, cm.HasColor("photo.jpg", ColorRed))
	})

	t.Run("remove last deletes key", func(t *testing.T) {
		cm := ColorMap{"photo.jpg": {ColorRed}}
		cm.ToggleColor("photo.jpg", ColorRed)
		_, exists := cm["photo.jpg"]
		assert.False(t, exists)
	})

	t.Run("multiple colors per file", func(t *testing.T) {
		cm := make(ColorMap)
		cm.ToggleColor("photo.jpg", ColorRed)
		cm.ToggleColor("photo.jpg", ColorGreen)
		cm.ToggleColor("photo.jpg", ColorBlue)
		assert.True(t, cm.HasColor("photo.jpg", ColorRed))
		assert.True(t, cm.HasColor("photo.jpg", ColorGreen))
		assert.True(t, cm.HasColor("photo.jpg", ColorBlue))
	})

	t.Run("different files independent", func(t *testing.T) {
		cm := make(ColorMap)
		cm.ToggleColor("a.jpg", ColorRed)
		cm.ToggleColor("b.jpg", ColorGreen)
		assert.True(t, cm.HasColor("a.jpg", ColorRed))
		assert.False(t, cm.HasColor("a.jpg", ColorGreen))
		assert.False(t, cm.HasColor("b.jpg", ColorRed))
		assert.True(t, cm.HasColor("b.jpg", ColorGreen))
	})
}

func TestColorMap_RemoveLabels(t *testing.T) {
	t.Parallel()

	t.Run("removes one of several colors", func(t *testing.T) {
		cm := ColorMap{"photo.jpg": {ColorRed, ColorGreen}}
		cm.RemoveLabels("photo.jpg", []ColorLabel{ColorRed})
		assert.False(t, cm.HasColor("photo.jpg", ColorRed))
		assert.True(t, cm.HasColor("photo.jpg", ColorGreen))
	})

	t.Run("removes multiple colors at once", func(t *testing.T) {
		cm := ColorMap{"photo.jpg": {ColorRed, ColorGreen, ColorBlue}}
		cm.RemoveLabels("photo.jpg", []ColorLabel{ColorRed, ColorGreen})
		assert.False(t, cm.HasColor("photo.jpg", ColorRed))
		assert.False(t, cm.HasColor("photo.jpg", ColorGreen))
		assert.True(t, cm.HasColor("photo.jpg", ColorBlue))
	})

	t.Run("deletes key when entry becomes empty", func(t *testing.T) {
		cm := ColorMap{"photo.jpg": {ColorRed, ColorGreen}}
		cm.RemoveLabels("photo.jpg", []ColorLabel{ColorRed, ColorGreen})
		_, exists := cm["photo.jpg"]
		assert.False(t, exists)
	})

	t.Run("no-op for absent filename", func(t *testing.T) {
		cm := ColorMap{"other.jpg": {ColorRed}}
		cm.RemoveLabels("photo.jpg", []ColorLabel{ColorRed})
		assert.True(t, cm.HasColor("other.jpg", ColorRed))
		_, exists := cm["photo.jpg"]
		assert.False(t, exists)
	})

	t.Run("no-op for color not present", func(t *testing.T) {
		cm := ColorMap{"photo.jpg": {ColorGreen}}
		cm.RemoveLabels("photo.jpg", []ColorLabel{ColorRed})
		assert.True(t, cm.HasColor("photo.jpg", ColorGreen))
	})
}

func TestLoadColors(t *testing.T) {
	t.Parallel()

	t.Run("no file returns empty", func(t *testing.T) {
		dir := t.TempDir()
		cm, err := LoadColors(dir)
		require.NoError(t, err)
		assert.Empty(t, cm)
	})

	t.Run("valid file", func(t *testing.T) {
		dir := t.TempDir()
		data := `{"photo.jpg":["red","green"]}`
		require.NoError(t, os.WriteFile(filepath.Join(dir, ColorsFileName), []byte(data), 0600))

		cm, err := LoadColors(dir)
		require.NoError(t, err)
		assert.True(t, cm.HasColor("photo.jpg", ColorRed))
		assert.True(t, cm.HasColor("photo.jpg", ColorGreen))
		assert.False(t, cm.HasColor("photo.jpg", ColorBlue))
	})

	t.Run("invalid JSON", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ColorsFileName), []byte("{bad json"), 0600))

		_, err := LoadColors(dir)
		assert.Error(t, err)
	})
}

func TestSaveColors(t *testing.T) {
	t.Parallel()

	t.Run("creates file", func(t *testing.T) {
		dir := t.TempDir()
		cm := ColorMap{"photo.jpg": {ColorRed}}
		require.NoError(t, SaveColors(dir, cm))

		_, err := os.Stat(filepath.Join(dir, ColorsFileName))
		assert.NoError(t, err)
	})

	t.Run("roundtrip", func(t *testing.T) {
		dir := t.TempDir()
		cm := ColorMap{
			"a.jpg": {ColorRed, ColorGreen},
			"b.jpg": {ColorBlue},
		}
		require.NoError(t, SaveColors(dir, cm))

		loaded, err := LoadColors(dir)
		require.NoError(t, err)
		assert.True(t, loaded.HasColor("a.jpg", ColorRed))
		assert.True(t, loaded.HasColor("a.jpg", ColorGreen))
		assert.True(t, loaded.HasColor("b.jpg", ColorBlue))
		assert.False(t, loaded.HasColor("b.jpg", ColorRed))
	})
}
