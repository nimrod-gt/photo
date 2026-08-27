package imaging

import (
	"errors"
	"image"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"photo/internal/core/model"
)

func stubProvider(load func(string) (image.Image, error)) *Provider {
	p := NewProvider(NewExifService())
	if load != nil {
		p.loader.loadImage = stubLoadImage(load)
	}
	return p
}

func TestImageProvider_Get(t *testing.T) {
	t.Parallel()

	t.Run("returns thumbnail for small size", func(t *testing.T) {
		thumb := fakeImage(160, 120)
		p := stubProvider(nil)
		p.thumbnails.Store("/photo.jpg", thumbEntry{img: thumb})

		img, err := p.Get("/photo.jpg", 160)
		require.NoError(t, err)
		assert.Equal(t, thumb, img)
	})

	t.Run("delegates to loader for large size", func(t *testing.T) {
		expected := fakeImage(2000, 1500)
		p := stubProvider(func(string) (image.Image, error) { return expected, nil })

		img, err := p.Get("/photo.jpg", 2000)
		require.NoError(t, err)
		assert.Equal(t, expected, img)
	})

	t.Run("auto-generates thumbnail after load", func(t *testing.T) {
		p := stubProvider(func(string) (image.Image, error) { return fakeImage(500, 400), nil })

		_, err := p.Get("/photo.jpg", 500)
		require.NoError(t, err)

		thumb := p.Thumbnail("/photo.jpg")
		require.NotNil(t, thumb)
		assert.LessOrEqual(t, thumb.Bounds().Dx(), maxThumbnailDim)
		assert.LessOrEqual(t, thumb.Bounds().Dy(), maxThumbnailDim)
	})

	t.Run("small size falls through to loader when no thumbnail", func(t *testing.T) {
		expected := fakeImage(100, 80)
		p := stubProvider(func(string) (image.Image, error) { return expected, nil })

		img, err := p.Get("/photo.jpg", 100)
		require.NoError(t, err)
		assert.Equal(t, expected, img)
	})

	t.Run("skips thumbnail too small for requested size", func(t *testing.T) {
		p := stubProvider(func(string) (image.Image, error) { return fakeImage(300, 200), nil })
		p.thumbnails.Store("/photo.jpg", thumbEntry{img: fakeImage(80, 60)})

		img, err := p.Get("/photo.jpg", 150)
		require.NoError(t, err)
		assert.Greater(t, img.Bounds().Dx(), 80)
	})
}

func TestImageProvider_Peek(t *testing.T) {
	t.Parallel()

	t.Run("returns thumbnail for small size", func(t *testing.T) {
		thumb := fakeImage(160, 120)
		p := stubProvider(nil)
		p.thumbnails.Store("/photo.jpg", thumbEntry{img: thumb})

		img := p.Peek("/photo.jpg", 160)
		assert.Equal(t, thumb, img)
	})

	t.Run("delegates to loader for large size", func(t *testing.T) {
		expected := fakeImage(50, 50)
		p := stubProvider(func(string) (image.Image, error) { return expected, nil })
		_, _ = p.loader.Get("/photo.jpg", 2000)

		img := p.Peek("/photo.jpg", 2000)
		assert.Equal(t, expected, img)
	})

	t.Run("returns nil when nothing cached", func(t *testing.T) {
		p := stubProvider(nil)
		assert.Nil(t, p.Peek("/photo.jpg", 2000))
	})

	t.Run("skips thumbnail too small for requested size", func(t *testing.T) {
		p := stubProvider(nil)
		p.thumbnails.Store("/photo.jpg", thumbEntry{img: fakeImage(80, 60)})

		assert.Nil(t, p.Peek("/photo.jpg", 150))
	})
}

func TestImageProvider_LoadFolder(t *testing.T) {
	t.Parallel()

	t.Run("non-JPEG files get nil thumbnail", func(t *testing.T) {
		p := stubProvider(nil)
		photos := []model.Photo{
			{ImagePath: "/tmp/test.png", Name: "test.png"},
			{ImagePath: "/tmp/test.arw", Name: "test.arw"},
		}

		var mu sync.Mutex
		results := make(map[int]bool)
		done := make(chan struct{})

		p.LoadFolder(photos,
			func(index int, meta LoadedMeta) {
				mu.Lock()
				results[index] = true
				assert.Equal(t, LoadedMeta{}, meta)
				mu.Unlock()
			},
			func() { close(done) },
		)

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for completion")
		}

		mu.Lock()
		defer mu.Unlock()
		assert.Len(t, results, 2)
		assert.True(t, results[0])
		assert.True(t, results[1])
	})

	t.Run("cancellation via repeated calls", func(t *testing.T) {
		p := stubProvider(nil)
		photos := make([]model.Photo, 100)
		for i := range photos {
			photos[i] = model.Photo{ImagePath: "/tmp/test.png", Name: "test.png"}
		}

		var callCount1 int
		var mu sync.Mutex

		p.LoadFolder(photos,
			func(int, LoadedMeta) {
				mu.Lock()
				callCount1++
				mu.Unlock()
			},
			nil,
		)

		done := make(chan struct{})
		p.LoadFolder(nil,
			func(int, LoadedMeta) { t.Fatal("should not be called") },
			func() { close(done) },
		)

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for completion")
		}
	})

	t.Run("clears previous state", func(t *testing.T) {
		p := stubProvider(func(string) (image.Image, error) { return fakeImage(50, 50), nil })
		_, _ = p.loader.Get("/old.jpg", 2000)
		p.thumbnails.Store("/old.jpg", thumbEntry{img: fakeImage(120, 90)})

		done := make(chan struct{})
		p.LoadFolder(nil,
			func(int, LoadedMeta) {},
			func() { close(done) },
		)

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out")
		}

		assert.Nil(t, p.loader.Peek("/old.jpg", 2000))
		assert.Nil(t, p.Thumbnail("/old.jpg"))
	})

	t.Run("empty photos calls onComplete immediately", func(t *testing.T) {
		p := stubProvider(nil)
		done := make(chan struct{})
		p.LoadFolder(nil,
			func(int, LoadedMeta) { t.Fatal("should not be called") },
			func() { close(done) },
		)

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for completion")
		}
	})

	t.Run("invalid JPEG calls onLoaded with nil thumbnail", func(t *testing.T) {
		p := stubProvider(nil)
		dir := t.TempDir()
		fakePath := dir + "/fake.jpg"
		require.NoError(t, os.WriteFile(fakePath, []byte("not a jpeg"), 0600))

		photos := []model.Photo{
			{ImagePath: fakePath, Name: "fake.jpg"},
		}

		var mu sync.Mutex
		results := make(map[int]bool)
		done := make(chan struct{})

		p.LoadFolder(photos,
			func(index int, meta LoadedMeta) {
				mu.Lock()
				results[index] = true
				assert.Equal(t, LoadedMeta{}, meta)
				mu.Unlock()
			},
			func() { close(done) },
		)

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for completion")
		}

		mu.Lock()
		defer mu.Unlock()
		assert.Len(t, results, 1)
		assert.True(t, results[0])
	})
}

func TestImageProvider_Preload(t *testing.T) {
	t.Parallel()

	t.Run("generates thumbnails from preloaded images", func(t *testing.T) {
		p := stubProvider(func(string) (image.Image, error) { return fakeImage(500, 400), nil })

		loaded := make(chan string, 1)
		p.Preload([]string{"/photo.jpg"}, 500, func(path string) {
			loaded <- path
		})

		select {
		case path := <-loaded:
			assert.Equal(t, "/photo.jpg", path)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for preload")
		}

		thumb := p.Thumbnail("/photo.jpg")
		require.NotNil(t, thumb)
		assert.LessOrEqual(t, thumb.Bounds().Dx(), maxThumbnailDim)
		assert.LessOrEqual(t, thumb.Bounds().Dy(), maxThumbnailDim)
	})
}

func TestImageProvider_Clear(t *testing.T) {
	t.Parallel()

	t.Run("purges everything", func(t *testing.T) {
		p := stubProvider(func(string) (image.Image, error) { return fakeImage(50, 50), nil })

		_, _ = p.Get("/photo.jpg", 2000)
		p.thumbnails.Store("/photo.jpg", thumbEntry{img: fakeImage(120, 90)})

		p.Clear()

		assert.Nil(t, p.loader.Peek("/photo.jpg", 2000))
		assert.Nil(t, p.Thumbnail("/photo.jpg"))
	})
}

func TestImageProvider_StoreThumbnail(t *testing.T) {
	t.Parallel()

	t.Run("decoded replaces exif thumbnail", func(t *testing.T) {
		p := stubProvider(nil)
		exifThumb := fakeImage(120, 90)
		p.thumbnails.Store("/photo.jpg", thumbEntry{img: exifThumb})

		p.storeThumbnail("/photo.jpg", fakeImage(4000, 3000))

		thumb := p.Thumbnail("/photo.jpg")
		require.NotNil(t, thumb)
		assert.NotEqual(t, exifThumb, thumb)
	})

	t.Run("does not overwrite decoded thumbnail", func(t *testing.T) {
		p := stubProvider(nil)
		decoded := fakeImage(120, 90)
		p.thumbnails.Store("/photo.jpg", thumbEntry{img: decoded, decoded: true})

		p.storeThumbnail("/photo.jpg", fakeImage(4000, 3000))

		thumb := p.Thumbnail("/photo.jpg")
		assert.Equal(t, decoded, thumb)
	})

	t.Run("stores new thumbnail when none exists", func(t *testing.T) {
		p := stubProvider(nil)

		p.storeThumbnail("/photo.jpg", fakeImage(4000, 3000))

		thumb := p.Thumbnail("/photo.jpg")
		require.NotNil(t, thumb)
		assert.LessOrEqual(t, thumb.Bounds().Dx(), maxThumbnailDim)
		assert.LessOrEqual(t, thumb.Bounds().Dy(), maxThumbnailDim)
	})

	t.Run("exif thumbnail does not clobber decoded in LoadFolder path", func(t *testing.T) {
		p := stubProvider(nil)
		decoded := fakeImage(120, 90)
		p.thumbnails.Store("/photo.jpg", thumbEntry{img: decoded, decoded: true})

		p.thumbnails.LoadOrStore("/photo.jpg", thumbEntry{img: fakeImage(100, 75)})

		thumb := p.Thumbnail("/photo.jpg")
		assert.Equal(t, decoded, thumb)
	})
}

func TestImageProvider_Orientation(t *testing.T) {
	t.Parallel()

	// nothing seeds an orientation any more: the JPEG decoder reads it out of
	// the file on every load
	t.Run("rotates a JPEG from its own EXIF on Get", func(t *testing.T) {
		path := writeJPEGSizedWithTags(t, t.TempDir(), "rotated.jpg", 4, 2, map[string]any{
			"Orientation": []uint16{6},
		})
		p := NewProvider(NewExifService())

		img, err := p.Get(path, 2000)
		require.NoError(t, err)
		assert.Equal(t, 2, img.Bounds().Dx())
		assert.Equal(t, 4, img.Bounds().Dy())
	})
}

func TestImageProvider_PeekThumbnailFitBox(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		thumb image.Image
		size  int
		found bool
	}{
		{"landscape thumb satisfies its fit size", fakeImage(160, 120), 160, true},
		{"landscape thumb satisfies smaller size", fakeImage(160, 120), 100, true},
		{"small thumb rejected for larger size", fakeImage(80, 60), 160, false},
		{"size above thumbnail limit bypasses store", fakeImage(160, 120), maxThumbnailDim + 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := stubProvider(nil)
			p.thumbnails.Store("/photo.jpg", thumbEntry{img: tt.thumb})

			img := p.peekThumbnail("/photo.jpg", tt.size)
			if tt.found {
				assert.Equal(t, tt.thumb, img)
			} else {
				assert.Nil(t, img)
			}
		})
	}
}

func stockProvider(t *testing.T, loaded map[string]StockInfo, read func(model.Photo) (StockInfo, error)) *Provider {
	t.Helper()

	p := NewProvider(NewExifService())
	p.loader.loadImage = stubLoadWithStock(func(path string) (LoadedImage, error) {
		return LoadedImage{Image: fakeImage(50, 50), Stock: loaded[path]}, nil
	})
	if read != nil {
		p.readStock = read
	}
	return p
}

func rawPhoto(t *testing.T, dir, name string, sidecar model.Tags) model.Photo {
	t.Helper()

	photo := model.Photo{
		ImagePath: filepath.Join(dir, name+".jpg"),
		RAWPath:   filepath.Join(dir, name+".ARW"),
		Name:      name + ".jpg",
	}
	require.NoError(t, WriteSidecar(model.SidecarPath(photo.RAWPath), sidecar))
	return photo
}

func TestImageProvider_StockInfo(t *testing.T) {
	t.Parallel()

	t.Run("serves the tags the image was loaded with", func(t *testing.T) {
		photo := model.Photo{ImagePath: "/photo.jpg", Name: "photo.jpg"}
		p := stockProvider(t, map[string]StockInfo{"/photo.jpg": stockOfTitle("bay")},
			func(model.Photo) (StockInfo, error) {
				t.Fatal("the files must not be read again")
				return StockInfo{}, nil
			})
		_, err := p.Get(photo.ImagePath, 2000)
		require.NoError(t, err)

		info, err := p.StockInfo(photo)
		require.NoError(t, err)
		assert.Equal(t, "bay", info.Tags.Title)
	})

	t.Run("reads the files when the image is not cached", func(t *testing.T) {
		photo := model.Photo{ImagePath: "/photo.jpg", Name: "photo.jpg"}
		var reads atomic.Int64
		p := stockProvider(t, nil, func(model.Photo) (StockInfo, error) {
			reads.Add(1)
			return stockOfTitle("bay"), nil
		})

		info, err := p.StockInfo(photo)
		require.NoError(t, err)
		assert.Equal(t, "bay", info.Tags.Title)

		info, err = p.StockInfo(photo)
		require.NoError(t, err)
		assert.Equal(t, "bay", info.Tags.Title)
		assert.Equal(t, int64(1), reads.Load())
	})

	t.Run("fills a JPEG without tags from the sidecar", func(t *testing.T) {
		dir := t.TempDir()
		photo := rawPhoto(t, dir, "a", model.Tags{Title: "cove", Keywords: []string{"rock"}})
		p := stockProvider(t, nil, nil)
		_, err := p.Get(photo.ImagePath, 2000)
		require.NoError(t, err)

		info, err := p.StockInfo(photo)
		require.NoError(t, err)
		assert.Equal(t, "cove", info.Tags.Title)
		assert.Equal(t, []string{"rock"}, info.Tags.Keywords)
	})

	t.Run("the sidecar beats the tags of the JPEG", func(t *testing.T) {
		dir := t.TempDir()
		photo := rawPhoto(t, dir, "a", model.Tags{Title: "cove"})
		p := stockProvider(t, map[string]StockInfo{
			filepath.Join(dir, "a.jpg"): {Tags: model.Tags{Title: "bay", Keywords: []string{"sea"}}},
		}, nil)
		_, err := p.Get(photo.ImagePath, 2000)
		require.NoError(t, err)

		info, err := p.StockInfo(photo)
		require.NoError(t, err)
		assert.Equal(t, "cove", info.Tags.Title)
		// the JPEG still fills what the sidecar leaves out
		assert.Equal(t, []string{"sea"}, info.Tags.Keywords)
	})

	t.Run("the sidecar is read once per entry", func(t *testing.T) {
		dir := t.TempDir()
		photo := rawPhoto(t, dir, "a", model.Tags{Title: "cove"})
		p := stockProvider(t, nil, nil)
		_, err := p.Get(photo.ImagePath, 2000)
		require.NoError(t, err)

		_, err = p.StockInfo(photo)
		require.NoError(t, err)
		require.NoError(t, os.Remove(model.SidecarPath(photo.RAWPath)))

		info, err := p.StockInfo(photo)
		require.NoError(t, err)
		assert.Equal(t, "cove", info.Tags.Title)
	})

	t.Run("a photo with no RAW pair keeps the tags of the JPEG", func(t *testing.T) {
		photo := model.Photo{ImagePath: "/photo.jpg", Name: "photo.jpg"}
		p := stockProvider(t, map[string]StockInfo{"/photo.jpg": stockOfTitle("bay")}, nil)
		_, err := p.Get(photo.ImagePath, 2000)
		require.NoError(t, err)

		info, err := p.StockInfo(photo)
		require.NoError(t, err)
		assert.Equal(t, "bay", info.Tags.Title)
	})

	t.Run("the sidecar of a photo with no RAW pair beats the tags of the JPEG", func(t *testing.T) {
		dir := t.TempDir()
		photo := model.Photo{ImagePath: filepath.Join(dir, "a.jpg"), Name: "a.jpg"}
		require.NoError(t, WriteSidecar(photo.SidecarPath(), model.Tags{Title: "cove"}))
		p := stockProvider(t, map[string]StockInfo{
			photo.ImagePath: {Tags: model.Tags{Title: "bay", Keywords: []string{"sea"}}},
		}, nil)
		_, err := p.Get(photo.ImagePath, 2000)
		require.NoError(t, err)

		info, err := p.StockInfo(photo)
		require.NoError(t, err)
		assert.Equal(t, "cove", info.Tags.Title)
		assert.Equal(t, []string{"sea"}, info.Tags.Keywords)
	})

	t.Run("concurrent calls read the files once", func(t *testing.T) {
		photo := model.Photo{ImagePath: "/photo.jpg", Name: "photo.jpg"}
		var reads atomic.Int64
		release := make(chan struct{})
		p := stockProvider(t, nil, func(model.Photo) (StockInfo, error) {
			reads.Add(1)
			<-release
			return stockOfTitle("bay"), nil
		})

		var wg sync.WaitGroup
		titles := make([]string, 4)
		for i := range titles {
			wg.Go(func() {
				info, err := p.StockInfo(photo)
				assert.NoError(t, err)
				titles[i] = info.Tags.Title
			})
		}
		time.Sleep(50 * time.Millisecond)
		close(release)
		wg.Wait()

		assert.Equal(t, int64(1), reads.Load())
		for _, title := range titles {
			assert.Equal(t, "bay", title)
		}
	})

	t.Run("a failed read is reported and not cached", func(t *testing.T) {
		photo := model.Photo{ImagePath: "/photo.jpg", Name: "photo.jpg"}
		var reads atomic.Int64
		p := stockProvider(t, nil, func(model.Photo) (StockInfo, error) {
			reads.Add(1)
			return StockInfo{}, errors.New("unreadable")
		})

		_, err := p.StockInfo(photo)
		require.Error(t, err)

		_, ok := p.PeekStockInfo(photo.ImagePath)
		assert.False(t, ok)

		_, err = p.StockInfo(photo)
		require.Error(t, err)
		assert.Equal(t, int64(2), reads.Load())
	})
}

func TestImageProvider_PeekStockInfo(t *testing.T) {
	t.Parallel()

	// the tags of a RAW-paired photo are only half read until the sidecar is
	// folded in, and a caller that cannot wait must not be handed them
	t.Run("misses tags that are not whole yet", func(t *testing.T) {
		dir := t.TempDir()
		photo := rawPhoto(t, dir, "a", model.Tags{Title: "cove"})
		p := stockProvider(t, map[string]StockInfo{filepath.Join(dir, "a.jpg"): stockOfTitle("bay")}, nil)
		_, err := p.Get(photo.ImagePath, 2000)
		require.NoError(t, err)

		_, ok := p.PeekStockInfo(photo.ImagePath)
		assert.False(t, ok)

		_, err = p.StockInfo(photo)
		require.NoError(t, err)

		info, ok := p.PeekStockInfo(photo.ImagePath)
		require.True(t, ok)
		assert.Equal(t, "cove", info.Tags.Title)
	})

	t.Run("misses an unknown path", func(t *testing.T) {
		p := stockProvider(t, nil, nil)

		_, ok := p.PeekStockInfo("/photo.jpg")
		assert.False(t, ok)
	})
}

func TestImageProvider_PeekStockDate(t *testing.T) {
	t.Parallel()

	taken := time.Date(2026, time.June, 14, 8, 0, 0, 0, time.UTC)

	t.Run("answers off a plain load, before the tags are whole", func(t *testing.T) {
		var reads atomic.Int64
		p := stockProvider(t, map[string]StockInfo{"/photo.jpg": {Taken: taken}}, func(model.Photo) (StockInfo, error) {
			reads.Add(1)
			return StockInfo{}, nil
		})
		_, err := p.Get("/photo.jpg", 2000)
		require.NoError(t, err)

		date, ok := p.PeekStockDate("/photo.jpg")

		require.True(t, ok)
		assert.Equal(t, taken, date)
		assert.Equal(t, int64(0), reads.Load())
		_, complete := p.PeekStockInfo("/photo.jpg")
		assert.False(t, complete, "the tags of the entry are not whole yet")
	})

	t.Run("misses an unknown path", func(t *testing.T) {
		p := stockProvider(t, nil, nil)

		_, ok := p.PeekStockDate("/photo.jpg")

		assert.False(t, ok)
	})

	t.Run("survives a store that carries no date", func(t *testing.T) {
		p := stockProvider(t, map[string]StockInfo{"/photo.jpg": {Taken: taken}}, nil)
		_, err := p.Get("/photo.jpg", 2000)
		require.NoError(t, err)

		p.StoreStockInfo("/photo.jpg", stockOfTitle("bay"))

		date, ok := p.PeekStockDate("/photo.jpg")
		require.True(t, ok)
		assert.Equal(t, taken, date)
	})
}

func TestImageProvider_StoreStockInfo(t *testing.T) {
	t.Parallel()

	// tags generated for a photo and not saved anywhere live in the cache
	// alone, and re-reading the sidecar would overwrite them
	t.Run("what the app stored is never read over", func(t *testing.T) {
		dir := t.TempDir()
		photo := rawPhoto(t, dir, "a", model.Tags{Title: "cove"})
		p := stockProvider(t, nil, nil)

		p.StoreStockInfo(photo.ImagePath, stockOfTitle("bay"))

		info, ok := p.PeekStockInfo(photo.ImagePath)
		require.True(t, ok)
		assert.Equal(t, "bay", info.Tags.Title)

		info, err := p.StockInfo(photo)
		require.NoError(t, err)
		assert.Equal(t, "bay", info.Tags.Title)
	})
}

func TestImageProvider_StoreStockInfoDuringARead(t *testing.T) {
	t.Parallel()

	t.Run("a read that finishes after the save does not overwrite it", func(t *testing.T) {
		photo := model.Photo{ImagePath: "/photo.jpg", Name: "photo.jpg"}
		started, release := make(chan struct{}), make(chan struct{})
		p := stockProvider(t, nil, func(model.Photo) (StockInfo, error) {
			close(started)
			<-release
			return stockOfTitle("file"), nil
		})

		var wg sync.WaitGroup
		var read StockInfo
		wg.Go(func() {
			var err error
			read, err = p.StockInfo(photo)
			assert.NoError(t, err)
		})
		<-started
		p.StoreStockInfo(photo.ImagePath, stockOfTitle("generated"))
		close(release)
		wg.Wait()

		info, ok := p.PeekStockInfo(photo.ImagePath)
		require.True(t, ok)
		assert.Equal(t, "generated", info.Tags.Title)
		assert.Equal(t, "generated", read.Tags.Title, "the overtaken read answered with the file it had already replaced")
	})
}

func TestImageProvider_Forget(t *testing.T) {
	t.Parallel()

	t.Run("drops the image, its tags and its thumbnail", func(t *testing.T) {
		p := stockProvider(t, map[string]StockInfo{"/photo.jpg": stockOfTitle("bay")}, nil)
		_, err := p.Get("/photo.jpg", 2000)
		require.NoError(t, err)
		p.StoreStockInfo("/photo.jpg", stockOfTitle("bay"))
		require.NotNil(t, p.Thumbnail("/photo.jpg"))

		p.Forget("/photo.jpg")

		assert.Nil(t, p.Peek("/photo.jpg", 2000))
		assert.Nil(t, p.Thumbnail("/photo.jpg"))
		_, ok := p.PeekStockInfo("/photo.jpg")
		assert.False(t, ok)
	})

	t.Run("a read that finishes after the delete adds nothing back", func(t *testing.T) {
		photo := model.Photo{ImagePath: "/photo.jpg", Name: "photo.jpg"}
		started, release := make(chan struct{}), make(chan struct{})
		p := stockProvider(t, nil, func(model.Photo) (StockInfo, error) {
			close(started)
			<-release
			return stockOfTitle("bay"), nil
		})

		var wg sync.WaitGroup
		wg.Go(func() {
			_, err := p.StockInfo(photo)
			assert.NoError(t, err)
		})
		<-started
		p.Forget(photo.ImagePath)
		close(release)
		wg.Wait()

		_, ok := p.PeekStockInfo(photo.ImagePath)
		assert.False(t, ok)
	})
}

func TestImageProvider_StockInfoAcrossAReload(t *testing.T) {
	t.Parallel()

	// the bigger image is read from the same JPEG, which knows nothing of the
	// sidecar: its own title must not speak over the one already folded in
	t.Run("the sidecar still wins after a reload", func(t *testing.T) {
		dir := t.TempDir()
		photo := rawPhoto(t, dir, "a", model.Tags{Title: "cove"})
		p := stockProvider(t, map[string]StockInfo{filepath.Join(dir, "a.jpg"): stockOfTitle("bay")}, nil)
		_, err := p.Get(photo.ImagePath, 100)
		require.NoError(t, err)
		_, err = p.StockInfo(photo)
		require.NoError(t, err)

		_, err = p.Get(photo.ImagePath, 2000)
		require.NoError(t, err)

		info, ok := p.PeekStockInfo(photo.ImagePath)
		require.True(t, ok)
		assert.Equal(t, "cove", info.Tags.Title)
	})

	t.Run("stored tags survive a reload", func(t *testing.T) {
		dir := t.TempDir()
		photo := rawPhoto(t, dir, "a", model.Tags{Title: "cove"})
		p := stockProvider(t, map[string]StockInfo{filepath.Join(dir, "a.jpg"): stockOfTitle("bay")}, nil)
		_, err := p.Get(photo.ImagePath, 100)
		require.NoError(t, err)
		p.StoreStockInfo(photo.ImagePath, stockOfTitle("reef"))

		_, err = p.Get(photo.ImagePath, 2000)
		require.NoError(t, err)

		info, err := p.StockInfo(photo)
		require.NoError(t, err)
		assert.Equal(t, "reef", info.Tags.Title)
	})
}
