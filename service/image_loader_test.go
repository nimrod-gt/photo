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

func fakeImage(w, h int) image.Image {
	return image.NewNRGBA(image.Rect(0, 0, w, h))
}

func stubLoader(load func(string) (image.Image, error)) *ImageLoader {
	l := NewImageLoader()
	if load != nil {
		l.loadImage = load
	}
	return l
}

func TestImageLoader_Get(t *testing.T) {
	t.Run("returns image", func(t *testing.T) {
		expected := fakeImage(50, 50)
		l := stubLoader(func(string) (image.Image, error) { return expected, nil })

		img, err := l.Get("/photo.jpg", 2000)
		require.NoError(t, err)
		assert.Equal(t, expected, img)
	})

	t.Run("returns error", func(t *testing.T) {
		l := stubLoader(func(string) (image.Image, error) { return nil, errors.New("broken") })

		_, err := l.Get("/photo.jpg", 2000)
		require.Error(t, err)
		assert.Equal(t, "broken", err.Error())
	})

	t.Run("caches result", func(t *testing.T) {
		var calls atomic.Int32
		l := stubLoader(func(string) (image.Image, error) {
			calls.Add(1)
			return fakeImage(10, 10), nil
		})

		_, _ = l.Get("/photo.jpg", 2000)
		_, _ = l.Get("/photo.jpg", 2000)
		assert.Equal(t, int32(1), calls.Load())
	})

	t.Run("does not cache errors", func(t *testing.T) {
		var calls atomic.Int32
		l := stubLoader(func(string) (image.Image, error) {
			calls.Add(1)
			return nil, errors.New("fail")
		})

		_, _ = l.Get("/photo.jpg", 2000)
		_, _ = l.Get("/photo.jpg", 2000)
		assert.Equal(t, int32(2), calls.Load())
	})

	t.Run("larger cached image satisfies smaller request", func(t *testing.T) {
		var calls atomic.Int32
		l := stubLoader(func(string) (image.Image, error) {
			calls.Add(1)
			return fakeImage(50, 50), nil
		})

		_, _ = l.Get("/photo.jpg", 2000)
		img, err := l.Get("/photo.jpg", 500)
		require.NoError(t, err)
		assert.NotNil(t, img)
		assert.Equal(t, int32(1), calls.Load())
	})

	t.Run("smaller cached image triggers reload for larger request", func(t *testing.T) {
		var calls atomic.Int32
		l := stubLoader(func(string) (image.Image, error) {
			calls.Add(1)
			return fakeImage(50, 50), nil
		})

		_, _ = l.Get("/photo.jpg", 500)
		_, _ = l.Get("/photo.jpg", 2000)
		assert.Equal(t, int32(2), calls.Load())
	})
}

func TestImageLoader_Peek(t *testing.T) {
	t.Run("returns nil on miss", func(t *testing.T) {
		l := stubLoader(nil)
		assert.Nil(t, l.Peek("/photo.jpg", 2000))
	})

	t.Run("returns cached image", func(t *testing.T) {
		expected := fakeImage(10, 10)
		l := stubLoader(func(string) (image.Image, error) { return expected, nil })

		_, _ = l.Get("/photo.jpg", 2000)
		img := l.Peek("/photo.jpg", 2000)
		assert.Equal(t, expected, img)
	})

	t.Run("does not trigger load", func(t *testing.T) {
		var calls atomic.Int32
		l := stubLoader(func(string) (image.Image, error) {
			calls.Add(1)
			return fakeImage(10, 10), nil
		})

		l.Peek("/photo.jpg", 2000)
		assert.Equal(t, int32(0), calls.Load())
	})

	t.Run("larger cached satisfies smaller peek", func(t *testing.T) {
		l := stubLoader(func(string) (image.Image, error) { return fakeImage(50, 50), nil })

		_, _ = l.Get("/photo.jpg", 2000)
		assert.NotNil(t, l.Peek("/photo.jpg", 500))
	})

	t.Run("smaller cached misses larger peek", func(t *testing.T) {
		l := stubLoader(func(string) (image.Image, error) { return fakeImage(50, 50), nil })

		_, _ = l.Get("/photo.jpg", 500)
		assert.Nil(t, l.Peek("/photo.jpg", 2000))
	})
}

func TestImageLoader_Preload(t *testing.T) {
	t.Run("loads in background", func(t *testing.T) {
		loaded := make(chan string, 1)
		l := stubLoader(func(string) (image.Image, error) { return fakeImage(10, 10), nil })

		l.Preload([]string{"/photo.jpg"}, 2000, func(path string) {
			loaded <- path
		})

		select {
		case p := <-loaded:
			assert.Equal(t, "/photo.jpg", p)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for preload")
		}

		img := l.Peek("/photo.jpg", 2000)
		assert.NotNil(t, img)
	})

	t.Run("skips cached", func(t *testing.T) {
		var calls atomic.Int32
		l := stubLoader(func(string) (image.Image, error) {
			calls.Add(1)
			return fakeImage(10, 10), nil
		})

		_, _ = l.Get("/photo.jpg", 2000)
		assert.Equal(t, int32(1), calls.Load())

		done := make(chan struct{})
		l.Preload([]string{"/photo.jpg"}, 2000, func(string) {
			close(done)
		})

		time.Sleep(50 * time.Millisecond)
		assert.Equal(t, int32(1), calls.Load())
	})

	t.Run("calls onLoaded per path", func(t *testing.T) {
		l := stubLoader(func(string) (image.Image, error) { return fakeImage(10, 10), nil })

		var mu sync.Mutex
		var loaded []string
		done := make(chan struct{})

		l.Preload([]string{"/a.jpg", "/b.jpg"}, 2000, func(path string) {
			mu.Lock()
			loaded = append(loaded, path)
			if len(loaded) == 2 {
				close(done)
			}
			mu.Unlock()
		})

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out")
		}

		mu.Lock()
		assert.Len(t, loaded, 2)
		assert.Contains(t, loaded, "/a.jpg")
		assert.Contains(t, loaded, "/b.jpg")
		mu.Unlock()
	})
}

func TestImageLoader_Dedup(t *testing.T) {
	t.Run("concurrent Get single load", func(t *testing.T) {
		var calls atomic.Int32
		started := make(chan struct{})
		proceed := make(chan struct{})
		l := stubLoader(func(string) (image.Image, error) {
			if calls.Add(1) == 1 {
				close(started)
				<-proceed
			}
			return fakeImage(10, 10), nil
		})

		var wg sync.WaitGroup
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = l.Get("/photo.jpg", 2000)
			}()
		}

		<-started
		close(proceed)
		wg.Wait()
		assert.Equal(t, int32(1), calls.Load())
	})

	t.Run("Get waits on inflight Preload", func(t *testing.T) {
		proceed := make(chan struct{})
		expected := fakeImage(20, 20)
		l := stubLoader(func(string) (image.Image, error) {
			<-proceed
			return expected, nil
		})

		l.Preload([]string{"/photo.jpg"}, 2000, nil)
		time.Sleep(20 * time.Millisecond)
		close(proceed)

		img, err := l.Get("/photo.jpg", 2000)
		require.NoError(t, err)
		assert.Equal(t, expected, img)
	})
}

func TestImageLoader_Clear(t *testing.T) {
	t.Run("purges caches", func(t *testing.T) {
		var calls atomic.Int32
		l := stubLoader(func(string) (image.Image, error) {
			calls.Add(1)
			return fakeImage(10, 10), nil
		})

		_, _ = l.Get("/photo.jpg", 2000)
		assert.Equal(t, int32(1), calls.Load())

		l.Clear()

		_, _ = l.Get("/photo.jpg", 2000)
		assert.Equal(t, int32(2), calls.Load())
	})

	t.Run("cancels preloads via generation", func(t *testing.T) {
		proceed := make(chan struct{})
		var calls atomic.Int32
		l := stubLoader(func(string) (image.Image, error) {
			calls.Add(1)
			<-proceed
			return fakeImage(10, 10), nil
		})

		l.Preload([]string{"/a.jpg", "/b.jpg", "/c.jpg"}, 2000, nil)
		time.Sleep(20 * time.Millisecond)
		l.Clear()
		close(proceed)

		time.Sleep(50 * time.Millisecond)
		assert.Nil(t, l.Peek("/a.jpg", 2000))
		assert.Nil(t, l.Peek("/b.jpg", 2000))
		assert.Nil(t, l.Peek("/c.jpg", 2000))
	})
}

func TestImageLoader_LRU(t *testing.T) {
	l := NewImageLoader()
	l.loadImage = func(string) (image.Image, error) {
		return fakeImage(10, 10), nil
	}

	for i := range cacheSize + 10 {
		path := "/photo_" + string(rune('A'+i%26)) + string(rune('0'+i/26)) + ".jpg"
		_, _ = l.Get(path, 2000)
	}

	assert.Equal(t, cacheSize, l.cache.Len())
}
