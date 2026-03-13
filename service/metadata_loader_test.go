package service

import (
	"context"
	"image"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"photo/model"
)

func TestMetadataLoader_LoadAsync(t *testing.T) {
	exif := NewExifService()
	loader := NewMetadataLoader(exif)

	t.Run("non-JPEG files get nil thumbnail", func(t *testing.T) {
		photos := []model.Photo{
			{ImagePath: "/tmp/test.png", Name: "test.png"},
			{ImagePath: "/tmp/test.arw", Name: "test.arw"},
		}

		var mu sync.Mutex
		results := make(map[int]bool)
		done := make(chan struct{})

		loader.LoadAsync(context.Background(), photos,
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

	t.Run("context cancellation stops processing", func(t *testing.T) {
		photos := make([]model.Photo, 100)
		for i := range photos {
			photos[i] = model.Photo{ImagePath: "/tmp/test.png", Name: "test.png"}
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		var callCount int
		var mu sync.Mutex
		completed := make(chan struct{}, 1)

		exited := make(chan struct{})
		loader.LoadAsync(ctx, photos,
			func(index int, _ image.Image, _ bool) {
				mu.Lock()
				callCount++
				mu.Unlock()
			},
			func() {
				completed <- struct{}{}
			},
		)
		go func() {
			// goroutine exits without calling onComplete when context is cancelled
			// wait briefly for it to finish, then signal
			time.Sleep(50 * time.Millisecond)
			close(exited)
		}()

		<-exited

		mu.Lock()
		assert.Equal(t, 0, callCount)
		mu.Unlock()

		select {
		case <-completed:
			t.Fatal("onComplete should not be called when context is cancelled")
		default:
		}
	})

	t.Run("empty photos calls onComplete immediately", func(t *testing.T) {
		done := make(chan struct{})
		loader.LoadAsync(context.Background(), nil,
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
		dir := t.TempDir()
		fakePath := dir + "/fake.jpg"
		require.NoError(t, os.WriteFile(fakePath, []byte("not a jpeg"), 0600))

		photos := []model.Photo{
			{ImagePath: fakePath, Name: "fake.jpg"},
		}

		var mu sync.Mutex
		results := make(map[int]bool)
		done := make(chan struct{})

		loader.LoadAsync(context.Background(), photos,
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
