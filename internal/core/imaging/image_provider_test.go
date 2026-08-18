package imaging

import (
	"image"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"photo/internal/core/model"
)

func stubProvider(load func(string) (image.Image, error)) *Provider {
	p := NewProvider(NewExifService())
	if load != nil {
		p.loader.loadImage = load
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
			func(index int, thumbnail image.Image, favorite bool) {
				mu.Lock()
				results[index] = true
				assert.Nil(t, thumbnail)
				assert.False(t, favorite)
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
			func(index int, _ image.Image, _ bool) {
				mu.Lock()
				callCount1++
				mu.Unlock()
			},
			nil,
		)

		done := make(chan struct{})
		p.LoadFolder(nil,
			func(int, image.Image, bool) { t.Fatal("should not be called") },
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
			func(int, image.Image, bool) {},
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
			func(int, image.Image, bool) { t.Fatal("should not be called") },
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
			func(index int, thumbnail image.Image, favorite bool) {
				mu.Lock()
				results[index] = true
				assert.Nil(t, thumbnail)
				assert.False(t, favorite)
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

func TestImageProvider_SeededOrientation(t *testing.T) {
	t.Parallel()

	t.Run("applies cached orientation on Get", func(t *testing.T) {
		path := writeTestJPEG(t, 4, 2)
		p := NewProvider(NewExifService())
		p.orientations.Store(path, uint16(6))

		img, err := p.Get(path, 2000)
		require.NoError(t, err)
		assert.Equal(t, 2, img.Bounds().Dx())
		assert.Equal(t, 4, img.Bounds().Dy())
	})

	t.Run("Clear drops cached orientations", func(t *testing.T) {
		p := NewProvider(NewExifService())
		p.orientations.Store("/photo.jpg", uint16(6))

		p.Clear()

		_, ok := p.orientations.Load("/photo.jpg")
		assert.False(t, ok)
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
