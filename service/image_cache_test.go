package service

import (
	"errors"
	"image"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stubCache(loader func(string) (image.Image, error)) *ImageCache {
	c := NewImageCache(image.Point{X: 100, Y: 100})
	c.loadImage = loader
	return c
}

func fakeImage(w, h int) image.Image {
	return image.NewNRGBA(image.Rect(0, 0, w, h))
}

func TestImageCache_Load(t *testing.T) {
	t.Run("returns loaded image", func(t *testing.T) {
		expected := fakeImage(50, 50)
		c := stubCache(func(string) (image.Image, error) {
			return expected, nil
		})

		img, err := c.Load("/photo.jpg")
		require.NoError(t, err)
		assert.Equal(t, expected, img)
	})

	t.Run("returns error from loader", func(t *testing.T) {
		c := stubCache(func(string) (image.Image, error) {
			return nil, errors.New("broken")
		})

		_, err := c.Load("/photo.jpg")
		require.Error(t, err)
		assert.Equal(t, "broken", err.Error())
	})

	t.Run("caches result", func(t *testing.T) {
		var calls atomic.Int32
		c := stubCache(func(string) (image.Image, error) {
			calls.Add(1)
			return fakeImage(10, 10), nil
		})

		_, _ = c.Load("/photo.jpg")
		_, _ = c.Load("/photo.jpg")
		assert.Equal(t, int32(1), calls.Load())
	})

	t.Run("caches error", func(t *testing.T) {
		var calls atomic.Int32
		c := stubCache(func(string) (image.Image, error) {
			calls.Add(1)
			return nil, errors.New("fail")
		})

		_, _ = c.Load("/photo.jpg")
		_, err := c.Load("/photo.jpg")
		require.Error(t, err)
		assert.Equal(t, int32(1), calls.Load())
	})
}

func TestImageCache_Prefetch(t *testing.T) {
	t.Run("loads in background", func(t *testing.T) {
		started := make(chan struct{})
		proceed := make(chan struct{})
		c := stubCache(func(string) (image.Image, error) {
			close(started)
			<-proceed
			return fakeImage(10, 10), nil
		})

		c.Prefetch("/photo.jpg")
		<-started
		close(proceed)

		img, err := c.Load("/photo.jpg")
		require.NoError(t, err)
		assert.NotNil(t, img)
	})

	t.Run("skips already cached", func(t *testing.T) {
		var calls atomic.Int32
		c := stubCache(func(string) (image.Image, error) {
			calls.Add(1)
			return fakeImage(10, 10), nil
		})

		_, _ = c.Load("/photo.jpg")
		c.Prefetch("/photo.jpg")
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, int32(1), calls.Load())
	})

	t.Run("skips in-flight prefetch", func(t *testing.T) {
		var calls atomic.Int32
		started := make(chan struct{})
		c := stubCache(func(string) (image.Image, error) {
			calls.Add(1)
			close(started)
			time.Sleep(50 * time.Millisecond)
			return fakeImage(10, 10), nil
		})

		c.Prefetch("/photo.jpg")
		<-started
		c.Prefetch("/photo.jpg")
		time.Sleep(100 * time.Millisecond)
		assert.Equal(t, int32(1), calls.Load())
	})
}

func TestImageCache_WaitInFlight(t *testing.T) {
	proceed := make(chan struct{})
	expected := fakeImage(20, 20)
	c := stubCache(func(string) (image.Image, error) {
		<-proceed
		return expected, nil
	})

	c.Prefetch("/photo.jpg")
	go func() {
		time.Sleep(20 * time.Millisecond)
		close(proceed)
	}()

	img, err := c.Load("/photo.jpg")
	require.NoError(t, err)
	assert.Equal(t, expected, img)
}

func TestImageCache_EvictExcept(t *testing.T) {
	var calls atomic.Int32
	c := stubCache(func(string) (image.Image, error) {
		calls.Add(1)
		return fakeImage(10, 10), nil
	})

	_, _ = c.Load("/a.jpg")
	_, _ = c.Load("/b.jpg")
	_, _ = c.Load("/c.jpg")
	assert.Equal(t, int32(3), calls.Load())

	c.EvictExcept([]string{"/a.jpg", "/c.jpg"})

	_, _ = c.Load("/a.jpg")
	assert.Equal(t, int32(3), calls.Load())

	_, _ = c.Load("/b.jpg")
	assert.Equal(t, int32(4), calls.Load())

	_, _ = c.Load("/c.jpg")
	assert.Equal(t, int32(4), calls.Load())
}

func TestImageCache_Clear(t *testing.T) {
	var calls atomic.Int32
	c := stubCache(func(string) (image.Image, error) {
		calls.Add(1)
		return fakeImage(10, 10), nil
	})

	_, _ = c.Load("/a.jpg")
	_, _ = c.Load("/b.jpg")
	assert.Equal(t, int32(2), calls.Load())

	c.Clear()

	_, _ = c.Load("/a.jpg")
	assert.Equal(t, int32(3), calls.Load())
}

func TestImageCache_ConcurrentAccess(t *testing.T) {
	var calls atomic.Int32
	c := stubCache(func(path string) (image.Image, error) {
		calls.Add(1)
		time.Sleep(5 * time.Millisecond)
		return fakeImage(10, 10), nil
	})

	var wg sync.WaitGroup
	errs := make([]error, 20)
	imgs := make([]image.Image, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			imgs[idx], errs[idx] = c.Load("/photo.jpg")
		}(i)
	}
	wg.Wait()

	for i := 0; i < 20; i++ {
		require.NoError(t, errs[i])
		assert.NotNil(t, imgs[i])
	}
	assert.Equal(t, int32(1), calls.Load())
}
