package service

import (
	"sync"
	"testing"

	"photo/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makePhotos(names ...string) []model.Photo {
	photos := make([]model.Photo, len(names))
	for i, name := range names {
		photos[i] = model.Photo{JPEGPath: "/photos/" + name, Name: name}
	}
	return photos
}

func TestNavigator_SetPhotos(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		nav := NewNavigator()
		nav.SetPhotos(nil)
		assert.Equal(t, 0, nav.Count())
		_, ok := nav.Current()
		assert.False(t, ok)
	})

	t.Run("non-empty", func(t *testing.T) {
		nav := NewNavigator()
		nav.SetPhotos(makePhotos("a.jpg", "b.jpg"))
		assert.Equal(t, 2, nav.Count())
		p, ok := nav.Current()
		assert.True(t, ok)
		assert.Equal(t, "a.jpg", p.Name)
	})

	t.Run("reset", func(t *testing.T) {
		nav := NewNavigator()
		nav.SetPhotos(makePhotos("a.jpg", "b.jpg"))
		nav.Next()
		nav.SetPhotos(makePhotos("c.jpg"))
		p, ok := nav.Current()
		assert.True(t, ok)
		assert.Equal(t, "c.jpg", p.Name)
	})
}

func TestNavigator_Current(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		nav := NewNavigator()
		_, ok := nav.Current()
		assert.False(t, ok)
	})

	t.Run("valid", func(t *testing.T) {
		nav := NewNavigator()
		nav.SetPhotos(makePhotos("a.jpg"))
		p, ok := nav.Current()
		assert.True(t, ok)
		assert.Equal(t, "a.jpg", p.Name)
	})
}

func TestNavigator_Next(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		nav := NewNavigator()
		nav.SetPhotos(makePhotos("a.jpg", "b.jpg", "c.jpg"))
		p, idx, ok := nav.Next()
		assert.True(t, ok)
		assert.Equal(t, 1, idx)
		assert.Equal(t, "b.jpg", p.Name)
	})

	t.Run("boundary stays at last", func(t *testing.T) {
		nav := NewNavigator()
		nav.SetPhotos(makePhotos("a.jpg", "b.jpg"))
		nav.Next()
		p, idx, ok := nav.Next()
		assert.True(t, ok)
		assert.Equal(t, 1, idx)
		assert.Equal(t, "b.jpg", p.Name)
	})

	t.Run("empty", func(t *testing.T) {
		nav := NewNavigator()
		_, _, ok := nav.Next()
		assert.False(t, ok)
	})
}

func TestNavigator_Previous(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		nav := NewNavigator()
		nav.SetPhotos(makePhotos("a.jpg", "b.jpg", "c.jpg"))
		nav.Next()
		nav.Next()
		p, idx, ok := nav.Previous()
		assert.True(t, ok)
		assert.Equal(t, 1, idx)
		assert.Equal(t, "b.jpg", p.Name)
	})

	t.Run("boundary stays at first", func(t *testing.T) {
		nav := NewNavigator()
		nav.SetPhotos(makePhotos("a.jpg", "b.jpg"))
		p, idx, ok := nav.Previous()
		assert.True(t, ok)
		assert.Equal(t, 0, idx)
		assert.Equal(t, "a.jpg", p.Name)
	})
}

func TestNavigator_GoTo(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		nav := NewNavigator()
		nav.SetPhotos(makePhotos("a.jpg", "b.jpg", "c.jpg"))
		p, idx, ok := nav.GoTo(2)
		assert.True(t, ok)
		assert.Equal(t, 2, idx)
		assert.Equal(t, "c.jpg", p.Name)
	})

	t.Run("negative ignored", func(t *testing.T) {
		nav := NewNavigator()
		nav.SetPhotos(makePhotos("a.jpg", "b.jpg"))
		p, idx, ok := nav.GoTo(-1)
		assert.True(t, ok)
		assert.Equal(t, 0, idx)
		assert.Equal(t, "a.jpg", p.Name)
	})

	t.Run("out of bounds ignored", func(t *testing.T) {
		nav := NewNavigator()
		nav.SetPhotos(makePhotos("a.jpg", "b.jpg"))
		p, idx, ok := nav.GoTo(99)
		assert.True(t, ok)
		assert.Equal(t, 0, idx)
		assert.Equal(t, "a.jpg", p.Name)
	})
}

func TestNavigator_FindIndex(t *testing.T) {
	nav := NewNavigator()
	nav.SetPhotos(makePhotos("a.jpg", "b.jpg", "c.jpg"))

	assert.Equal(t, 1, nav.FindIndex("/photos/b.jpg"))
	assert.Equal(t, -1, nav.FindIndex("/photos/missing.jpg"))
}

func TestNavigator_RemoveCurrent(t *testing.T) {
	t.Run("middle", func(t *testing.T) {
		nav := NewNavigator()
		nav.SetPhotos(makePhotos("a.jpg", "b.jpg", "c.jpg"))
		nav.GoTo(1)
		p, idx, photos, ok := nav.RemoveCurrent()
		assert.True(t, ok)
		assert.Equal(t, 1, idx)
		assert.Equal(t, "c.jpg", p.Name)
		assert.Len(t, photos, 2)
	})

	t.Run("last element", func(t *testing.T) {
		nav := NewNavigator()
		nav.SetPhotos(makePhotos("a.jpg", "b.jpg"))
		nav.GoTo(1)
		p, idx, photos, ok := nav.RemoveCurrent()
		assert.True(t, ok)
		assert.Equal(t, 0, idx)
		assert.Equal(t, "a.jpg", p.Name)
		assert.Len(t, photos, 1)
	})

	t.Run("only one", func(t *testing.T) {
		nav := NewNavigator()
		nav.SetPhotos(makePhotos("a.jpg"))
		_, _, _, ok := nav.RemoveCurrent()
		assert.False(t, ok)
		assert.Equal(t, 0, nav.Count())
	})

	t.Run("empty", func(t *testing.T) {
		nav := NewNavigator()
		_, _, _, ok := nav.RemoveCurrent()
		assert.False(t, ok)
	})
}

func TestNavigator_Count(t *testing.T) {
	nav := NewNavigator()
	assert.Equal(t, 0, nav.Count())
	nav.SetPhotos(makePhotos("a.jpg", "b.jpg"))
	assert.Equal(t, 2, nav.Count())
}

func TestNavigator_ConcurrentAccess(t *testing.T) {
	nav := NewNavigator()
	nav.SetPhotos(makePhotos("a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg"))

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(4)
		go func() {
			defer wg.Done()
			nav.Next()
		}()
		go func() {
			defer wg.Done()
			nav.Previous()
		}()
		go func() {
			defer wg.Done()
			nav.Current()
		}()
		go func() {
			defer wg.Done()
			nav.Count()
		}()
	}
	wg.Wait()

	_, ok := nav.Current()
	require.True(t, ok)
}
