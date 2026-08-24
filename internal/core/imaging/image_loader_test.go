package imaging

import (
	"errors"
	"image"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"photo/internal/core/model"
)

func fakeImage(w, h int) image.Image {
	return image.NewNRGBA(image.Rect(0, 0, w, h))
}

func stubLoader(load func(string) (image.Image, error)) *Loader {
	l := NewLoader(LoadImageWithStock)
	if load != nil {
		l.loadImage = stubLoadImage(load)
	}
	return l
}

// the real loadImage decodes straight to the size it is asked for, so a stub
// that ignored it would cache images the loader never caches
func stubLoadImage(load func(string) (image.Image, error)) LoadFunc {
	return stubLoadWithStock(func(path string) (LoadedImage, error) {
		img, err := load(path)
		return LoadedImage{Image: img}, err
	})
}

func stubLoadWithStock(load func(string) (LoadedImage, error)) LoadFunc {
	return func(path string, size int) (LoadedImage, error) {
		loaded, err := load(path)
		if err != nil {
			return LoadedImage{}, err
		}
		loaded.Image = DownscaleToFit(loaded.Image, image.Point{X: size, Y: size})
		return loaded, nil
	}
}

func TestImageLoader_Get(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
			wg.Go(func() {
				_, _ = l.Get("/photo.jpg", 2000)
			})
		}

		<-started
		close(proceed)
		wg.Wait()
		assert.Equal(t, int32(1), calls.Load())
	})

	t.Run("concurrent larger-size Gets dedup after small inflight", func(t *testing.T) {
		var calls atomic.Int32
		firstStarted := make(chan struct{})
		firstProceed := make(chan struct{})
		l := stubLoader(func(string) (image.Image, error) {
			if calls.Add(1) == 1 {
				close(firstStarted)
				<-firstProceed
			}
			return fakeImage(3000, 2000), nil
		})

		smallDone := make(chan struct{})
		go func() {
			defer close(smallDone)
			_, _ = l.Get("/photo.jpg", 100)
		}()
		<-firstStarted

		var wg sync.WaitGroup
		for range 5 {
			wg.Go(func() {
				img, err := l.Get("/photo.jpg", 2000)
				assert.NoError(t, err) //nolint:testifylint // require is unsafe outside the test goroutine
				assert.NotNil(t, img)
			})
		}

		time.Sleep(20 * time.Millisecond)
		close(firstProceed)
		<-smallDone
		wg.Wait()

		assert.Equal(t, int32(2), calls.Load())
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
	t.Parallel()

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
	t.Parallel()

	l := stubLoader(func(string) (image.Image, error) { return fakeImage(10, 10), nil })

	for i := range cacheMaxEntries + 10 {
		path := "/photo_" + string(rune('A'+i%26)) + string(rune('0'+i/26)) + ".jpg"
		_, _ = l.Get(path, 2000)
	}

	assert.Equal(t, cacheMaxEntries, l.cache.Len())
}

func TestImageLoader_ByteBudget(t *testing.T) {
	t.Parallel()

	imgBytes := imageBytes(fakeImage(10, 10))

	t.Run("evicts oldest when over budget", func(t *testing.T) {
		l := stubLoader(func(string) (image.Image, error) { return fakeImage(10, 10), nil })
		l.byteBudget = int64(imgBytes * 3)

		_, _ = l.Get("/a.jpg", 100)
		_, _ = l.Get("/b.jpg", 100)
		_, _ = l.Get("/c.jpg", 100)
		_, _ = l.Get("/d.jpg", 100)

		assert.Equal(t, 3, l.cache.Len())
		assert.LessOrEqual(t, l.cacheBytes.Load(), l.byteBudget)
		assert.Nil(t, l.Peek("/a.jpg", 100))
		assert.NotNil(t, l.Peek("/d.jpg", 100))
	})

	t.Run("keeps newest entry even if alone over budget", func(t *testing.T) {
		l := stubLoader(func(string) (image.Image, error) { return fakeImage(10, 10), nil })
		l.byteBudget = int64(imgBytes / 2)

		_, _ = l.Get("/a.jpg", 100)
		_, _ = l.Get("/b.jpg", 100)

		assert.Equal(t, 1, l.cache.Len())
		assert.NotNil(t, l.Peek("/b.jpg", 100))
	})

	t.Run("replacing entry accounts bytes correctly", func(t *testing.T) {
		sizes := map[int]image.Image{100: fakeImage(10, 10), 2000: fakeImage(50, 50)}
		l := stubLoader(nil)
		l.loadImage = stubLoadImage(func(string) (image.Image, error) { return fakeImage(10, 10), nil })

		l.addToCache("/a.jpg", cachedImage{img: sizes[100], size: 100})
		l.addToCache("/a.jpg", cachedImage{img: sizes[2000], size: 2000})

		assert.Equal(t, 1, l.cache.Len())
		assert.Equal(t, int64(imageBytes(sizes[2000])), l.cacheBytes.Load())
	})

	t.Run("Clear resets byte counter", func(t *testing.T) {
		l := stubLoader(func(string) (image.Image, error) { return fakeImage(10, 10), nil })

		_, _ = l.Get("/a.jpg", 100)
		_, _ = l.Get("/b.jpg", 100)
		assert.Positive(t, l.cacheBytes.Load())

		l.Clear()
		assert.Equal(t, int64(0), l.cacheBytes.Load())
	})
}

func TestImageBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		img      image.Image
		expected int
	}{
		{"NRGBA", image.NewNRGBA(image.Rect(0, 0, 10, 10)), 400},
		{"RGBA", image.NewRGBA(image.Rect(0, 0, 10, 10)), 400},
		{"Gray falls back to 4 bytes per pixel", image.NewGray(image.Rect(0, 0, 10, 10)), 400},
		// nothing decoded for the cache is subsampled today, but the fallback
		// would bill this one for 400 bytes rather than the 150 it holds
		{"YCbCr 4:2:0 is billed for its planes", image.NewYCbCr(image.Rect(0, 0, 10, 10), image.YCbCrSubsampleRatio420), 150},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, imageBytes(tt.img))
		})
	}
}

func TestImageLoader_ClampsNonPositiveSize(t *testing.T) {
	t.Parallel()

	t.Run("downscales to a single pixel", func(t *testing.T) {
		l := stubLoader(func(string) (image.Image, error) { return fakeImage(10, 10), nil })

		img, err := l.Get("/a.jpg", 0)
		require.NoError(t, err)
		assert.Equal(t, 1, img.Bounds().Dx())
		assert.Equal(t, 1, img.Bounds().Dy())
	})

	// caching the 1x1 result under the unclamped size would make it satisfy
	// every later lookup for a non-positive size
	t.Run("caches under the clamped size", func(t *testing.T) {
		l := stubLoader(func(string) (image.Image, error) { return fakeImage(10, 10), nil })

		_, err := l.Get("/a.jpg", 0)
		require.NoError(t, err)

		entry, ok := l.cache.Peek("/a.jpg")
		require.True(t, ok)
		assert.Equal(t, 1, entry.size)
	})
}

func stockOfTitle(title string) StockInfo {
	return StockInfo{Tags: model.Tags{Title: title, Keywords: []string{"sea"}}}
}

func TestImageLoader_Stock(t *testing.T) {
	t.Parallel()

	t.Run("Get caches the tags with the image", func(t *testing.T) {
		l := stubLoader(nil)
		l.loadImage = stubLoadWithStock(func(string) (LoadedImage, error) {
			return LoadedImage{Image: fakeImage(10, 10), Stock: stockOfTitle("bay")}, nil
		})

		_, err := l.Get("/a.jpg", 100)
		require.NoError(t, err)

		info, ok := l.PeekStock("/a.jpg")
		require.True(t, ok)
		assert.Equal(t, "bay", info.Tags.Title)
	})

	t.Run("Preload caches the tags with the image", func(t *testing.T) {
		l := stubLoader(nil)
		l.loadImage = stubLoadWithStock(func(string) (LoadedImage, error) {
			return LoadedImage{Image: fakeImage(10, 10), Stock: stockOfTitle("bay")}, nil
		})

		done := make(chan struct{})
		l.Preload([]string{"/a.jpg"}, 100, func(string) { close(done) })
		<-done

		info, ok := l.PeekStock("/a.jpg")
		require.True(t, ok)
		assert.Equal(t, "bay", info.Tags.Title)
	})

	// the second read asks for a bigger image, and a loader that returned no
	// tags on it would drop the ones the entry already holds
	t.Run("a reload at a larger size keeps the tags", func(t *testing.T) {
		var calls atomic.Int64
		l := stubLoader(nil)
		l.loadImage = stubLoadWithStock(func(string) (LoadedImage, error) {
			loaded := LoadedImage{Image: fakeImage(2000, 2000)}
			if calls.Add(1) == 1 {
				loaded.Stock = stockOfTitle("bay")
			}
			return loaded, nil
		})

		_, err := l.Get("/a.jpg", 100)
		require.NoError(t, err)
		_, err = l.Get("/a.jpg", 800)
		require.NoError(t, err)

		info, ok := l.PeekStock("/a.jpg")
		require.True(t, ok)
		assert.Equal(t, "bay", info.Tags.Title)
	})

	t.Run("tags that failed to read leave the image cached", func(t *testing.T) {
		l := stubLoader(nil)
		l.loadImage = stubLoadWithStock(func(string) (LoadedImage, error) {
			return LoadedImage{Image: fakeImage(10, 10), StockErr: errors.New("broken packet")}, nil
		})

		img, err := l.Get("/a.jpg", 100)
		require.NoError(t, err)
		assert.NotNil(t, img)
		assert.NotNil(t, l.Peek("/a.jpg", 100))

		_, ok := l.PeekStock("/a.jpg")
		assert.False(t, ok)
	})

	t.Run("StoreStock updates a cached image", func(t *testing.T) {
		l := stubLoader(func(string) (image.Image, error) { return fakeImage(10, 10), nil })

		_, err := l.Get("/a.jpg", 100)
		require.NoError(t, err)
		l.StoreStock("/a.jpg", stockOfTitle("bay"))

		info, ok := l.PeekStock("/a.jpg")
		require.True(t, ok)
		assert.Equal(t, "bay", info.Tags.Title)
		assert.NotNil(t, l.Peek("/a.jpg", 100))
	})

	t.Run("StoreStock keeps tags for a path with no image", func(t *testing.T) {
		l := stubLoader(nil)

		l.StoreStock("/a.jpg", stockOfTitle("bay"))

		info, ok := l.PeekStock("/a.jpg")
		require.True(t, ok)
		assert.Equal(t, "bay", info.Tags.Title)
	})

	// an entry holding tags alone has no image and a size no lookup can ask for
	t.Run("a tags-only entry is never handed out as an image", func(t *testing.T) {
		l := stubLoader(func(string) (image.Image, error) { return fakeImage(10, 10), nil })
		l.StoreStock("/a.jpg", stockOfTitle("bay"))

		assert.Nil(t, l.Peek("/a.jpg", 1))
		assert.Zero(t, l.cacheBytes.Load())

		img, err := l.Get("/a.jpg", 100)
		require.NoError(t, err)
		assert.NotNil(t, img)

		info, ok := l.PeekStock("/a.jpg")
		require.True(t, ok)
		assert.Equal(t, "bay", info.Tags.Title)
	})

	t.Run("PeekStock misses an unknown path", func(t *testing.T) {
		l := stubLoader(nil)

		_, ok := l.PeekStock("/a.jpg")
		assert.False(t, ok)
	})

	t.Run("Forget drops the entry", func(t *testing.T) {
		l := stubLoader(func(string) (image.Image, error) { return fakeImage(10, 10), nil })

		_, err := l.Get("/a.jpg", 100)
		require.NoError(t, err)
		l.StoreStock("/a.jpg", stockOfTitle("bay"))

		l.Forget("/a.jpg")

		_, ok := l.PeekStock("/a.jpg")
		assert.False(t, ok)
		assert.Nil(t, l.Peek("/a.jpg", 100))
		assert.Zero(t, l.cacheBytes.Load())
	})

	t.Run("Clear drops the tags", func(t *testing.T) {
		l := stubLoader(nil)
		l.StoreStock("/a.jpg", stockOfTitle("bay"))

		l.Clear()

		_, ok := l.PeekStock("/a.jpg")
		assert.False(t, ok)
	})
}

func TestImageLoader_StockOfAReload(t *testing.T) {
	t.Parallel()

	t.Run("what the file carries wins over the entry", func(t *testing.T) {
		var calls atomic.Int64
		l := stubLoader(nil)
		l.loadImage = stubLoadWithStock(func(string) (LoadedImage, error) {
			title := "bay"
			if calls.Add(1) > 1 {
				title = "cove"
			}
			return LoadedImage{Image: fakeImage(2000, 2000), Stock: stockOfTitle(title)}, nil
		})

		_, err := l.Get("/a.jpg", 100)
		require.NoError(t, err)
		_, err = l.Get("/a.jpg", 800)
		require.NoError(t, err)

		info, ok := l.PeekStock("/a.jpg")
		require.True(t, ok)
		assert.Equal(t, "cove", info.Tags.Title)
	})

	t.Run("the date of the entry survives a read that has none", func(t *testing.T) {
		taken := time.Date(2024, time.May, 1, 10, 0, 0, 0, time.UTC)
		l := stubLoader(nil)
		l.loadImage = stubLoadWithStock(func(string) (LoadedImage, error) {
			return LoadedImage{Image: fakeImage(10, 10)}, nil
		})
		l.StoreStock("/a.jpg", StockInfo{Tags: model.Tags{Title: "bay"}, Taken: taken})

		_, err := l.Get("/a.jpg", 100)
		require.NoError(t, err)

		info, ok := l.PeekStock("/a.jpg")
		require.True(t, ok)
		assert.Equal(t, taken, info.Taken)
	})
}

func TestImageLoader_StockKeywordsAreNotShared(t *testing.T) {
	t.Parallel()

	t.Run("editing the stored tags leaves the entry alone", func(t *testing.T) {
		l := stubLoader(nil)
		info := StockInfo{Tags: model.Tags{Title: "bay", Keywords: []string{"sea"}}}

		l.StoreStock("/a.jpg", info)
		info.Tags.Keywords[0] = "sky"

		cached, ok := l.PeekStock("/a.jpg")
		require.True(t, ok)
		assert.Equal(t, []string{"sea"}, cached.Tags.Keywords)
	})

	t.Run("editing the peeked tags leaves the entry alone", func(t *testing.T) {
		l := stubLoader(nil)
		l.StoreStock("/a.jpg", StockInfo{Tags: model.Tags{Keywords: []string{"sea"}}})

		peeked, ok := l.PeekStock("/a.jpg")
		require.True(t, ok)
		peeked.Tags.Keywords[0] = "sky"

		cached, ok := l.PeekStock("/a.jpg")
		require.True(t, ok)
		assert.Equal(t, []string{"sea"}, cached.Tags.Keywords)
	})
}
