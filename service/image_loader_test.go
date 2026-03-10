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

func stubLoader(loadFull, loadThumb func(string) (image.Image, error)) *ImageLoader {
	l := NewImageLoader(
		image.Point{X: 100, Y: 100},
		func() image.Point { return image.Point{X: 200, Y: 200} },
		nil,
	)
	if loadFull != nil {
		l.loadFull = loadFull
	}
	if loadThumb != nil {
		l.loadThumb = loadThumb
	}
	return l
}

func TestImageLoader_Get(t *testing.T) {
	t.Run("returns image", func(t *testing.T) {
		expected := fakeImage(50, 50)
		l := stubLoader(
			func(string) (image.Image, error) { return expected, nil },
			nil,
		)

		img, err := l.Get("/photo.jpg", SizeFull)
		require.NoError(t, err)
		assert.Equal(t, expected, img)
	})

	t.Run("returns error", func(t *testing.T) {
		l := stubLoader(
			func(string) (image.Image, error) { return nil, errors.New("broken") },
			nil,
		)

		_, err := l.Get("/photo.jpg", SizeFull)
		require.Error(t, err)
		assert.Equal(t, "broken", err.Error())
	})

	t.Run("caches result", func(t *testing.T) {
		var calls atomic.Int32
		l := stubLoader(
			func(string) (image.Image, error) {
				calls.Add(1)
				return fakeImage(10, 10), nil
			},
			nil,
		)

		_, _ = l.Get("/photo.jpg", SizeFull)
		_, _ = l.Get("/photo.jpg", SizeFull)
		assert.Equal(t, int32(1), calls.Load())
	})

	t.Run("does not cache errors", func(t *testing.T) {
		var calls atomic.Int32
		l := stubLoader(
			func(string) (image.Image, error) {
				calls.Add(1)
				return nil, errors.New("fail")
			},
			nil,
		)

		_, _ = l.Get("/photo.jpg", SizeFull)
		_, _ = l.Get("/photo.jpg", SizeFull)
		assert.Equal(t, int32(2), calls.Load())
	})

	t.Run("thumb and full are separate caches", func(t *testing.T) {
		thumbImg := fakeImage(10, 10)
		fullImg := fakeImage(50, 50)
		l := stubLoader(
			func(string) (image.Image, error) { return fullImg, nil },
			func(string) (image.Image, error) { return thumbImg, nil },
		)

		img1, _ := l.Get("/photo.jpg", SizeFull)
		img2, _ := l.Get("/photo.jpg", SizeThumb)
		assert.Equal(t, fullImg, img1)
		assert.Equal(t, thumbImg, img2)
	})
}

func TestImageLoader_Peek(t *testing.T) {
	t.Run("returns nil on miss", func(t *testing.T) {
		l := stubLoader(nil, nil)
		assert.Nil(t, l.Peek("/photo.jpg", SizeFull))
	})

	t.Run("returns cached image", func(t *testing.T) {
		expected := fakeImage(10, 10)
		l := stubLoader(
			func(string) (image.Image, error) { return expected, nil },
			nil,
		)

		_, _ = l.Get("/photo.jpg", SizeFull)
		img := l.Peek("/photo.jpg", SizeFull)
		assert.Equal(t, expected, img)
	})

	t.Run("does not trigger load", func(t *testing.T) {
		var calls atomic.Int32
		l := stubLoader(
			func(string) (image.Image, error) {
				calls.Add(1)
				return fakeImage(10, 10), nil
			},
			nil,
		)

		l.Peek("/photo.jpg", SizeFull)
		assert.Equal(t, int32(0), calls.Load())
	})
}

func TestImageLoader_Preload(t *testing.T) {
	t.Run("loads in background", func(t *testing.T) {
		loaded := make(chan string, 1)
		l := stubLoader(
			func(string) (image.Image, error) { return fakeImage(10, 10), nil },
			nil,
		)

		l.Preload([]string{"/photo.jpg"}, SizeFull, func(path string) {
			loaded <- path
		})

		select {
		case p := <-loaded:
			assert.Equal(t, "/photo.jpg", p)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for preload")
		}

		img := l.Peek("/photo.jpg", SizeFull)
		assert.NotNil(t, img)
	})

	t.Run("skips cached", func(t *testing.T) {
		var calls atomic.Int32
		l := stubLoader(
			func(string) (image.Image, error) {
				calls.Add(1)
				return fakeImage(10, 10), nil
			},
			nil,
		)

		_, _ = l.Get("/photo.jpg", SizeFull)
		assert.Equal(t, int32(1), calls.Load())

		done := make(chan struct{})
		l.Preload([]string{"/photo.jpg"}, SizeFull, func(string) {
			close(done)
		})

		time.Sleep(50 * time.Millisecond)
		assert.Equal(t, int32(1), calls.Load())
	})

	t.Run("calls onLoaded per path", func(t *testing.T) {
		l := stubLoader(
			func(string) (image.Image, error) { return fakeImage(10, 10), nil },
			nil,
		)

		var mu sync.Mutex
		var loaded []string
		done := make(chan struct{})

		l.Preload([]string{"/a.jpg", "/b.jpg"}, SizeFull, func(path string) {
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
		l := stubLoader(
			func(string) (image.Image, error) {
				if calls.Add(1) == 1 {
					close(started)
					<-proceed
				}
				return fakeImage(10, 10), nil
			},
			nil,
		)

		var wg sync.WaitGroup
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = l.Get("/photo.jpg", SizeFull)
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
		l := stubLoader(
			func(string) (image.Image, error) {
				<-proceed
				return expected, nil
			},
			nil,
		)

		l.Preload([]string{"/photo.jpg"}, SizeFull, nil)
		time.Sleep(20 * time.Millisecond)
		close(proceed)

		img, err := l.Get("/photo.jpg", SizeFull)
		require.NoError(t, err)
		assert.Equal(t, expected, img)
	})
}

func TestImageLoader_Clear(t *testing.T) {
	t.Run("purges caches", func(t *testing.T) {
		var calls atomic.Int32
		l := stubLoader(
			func(string) (image.Image, error) {
				calls.Add(1)
				return fakeImage(10, 10), nil
			},
			nil,
		)

		_, _ = l.Get("/photo.jpg", SizeFull)
		assert.Equal(t, int32(1), calls.Load())

		l.Clear()

		_, _ = l.Get("/photo.jpg", SizeFull)
		assert.Equal(t, int32(2), calls.Load())
	})

	t.Run("cancels preloads via generation", func(t *testing.T) {
		proceed := make(chan struct{})
		var calls atomic.Int32
		l := stubLoader(
			func(string) (image.Image, error) {
				calls.Add(1)
				<-proceed
				return fakeImage(10, 10), nil
			},
			nil,
		)

		l.Preload([]string{"/a.jpg", "/b.jpg", "/c.jpg"}, SizeFull, nil)
		time.Sleep(20 * time.Millisecond)
		l.Clear()
		close(proceed)

		time.Sleep(50 * time.Millisecond)
		assert.Nil(t, l.Peek("/a.jpg", SizeFull))
		assert.Nil(t, l.Peek("/b.jpg", SizeFull))
		assert.Nil(t, l.Peek("/c.jpg", SizeFull))
	})
}

func TestImageLoader_LRU(t *testing.T) {
	l := NewImageLoader(
		image.Point{X: 100, Y: 100},
		func() image.Point { return image.Point{X: 200, Y: 200} },
		nil,
	)
	l.loadFull = func(string) (image.Image, error) {
		return fakeImage(10, 10), nil
	}

	for i := range fullCacheSize + 10 {
		path := "/photo_" + string(rune('A'+i%26)) + string(rune('0'+i/26)) + ".jpg"
		_, _ = l.Get(path, SizeFull)
	}

	assert.Equal(t, fullCacheSize, l.fulls.Len())
}
