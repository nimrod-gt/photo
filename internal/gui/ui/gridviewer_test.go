package ui

import (
	"fmt"
	"image"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"photo/internal/core/model"
)

type gridPreloadCall struct {
	paths    []string
	size     int
	onLoaded func(string)
}

type fakeGridProvider struct {
	mu     sync.Mutex
	sized  image.Image
	peeked []int

	calls chan gridPreloadCall
	block chan struct{}
	gen   atomic.Uint64
}

func newFakeGridProvider() *fakeGridProvider {
	return &fakeGridProvider{calls: make(chan gridPreloadCall, 64)}
}

func (f *fakeGridProvider) Peek(_ string, size int) image.Image {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.peeked = append(f.peeked, size)
	return f.sized
}

func (f *fakeGridProvider) Thumbnail(string) image.Image { return nil }

func (f *fakeGridProvider) Preload(paths []string, size int, onLoaded func(string)) {
	f.calls <- gridPreloadCall{paths: paths, size: size, onLoaded: onLoaded}

	f.mu.Lock()
	block := f.block
	f.mu.Unlock()
	if block != nil {
		<-block
	}
}

// Holds every dispatch inside Preload until the channel is closed, so a test
// can make a tile miss while one is in flight.
func (f *fakeGridProvider) blockPreloads(block chan struct{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.block = block
}

func (f *fakeGridProvider) Gen() uint64 { return f.gen.Load() }

func (f *fakeGridProvider) BumpGen() { f.gen.Add(1) }

func (f *fakeGridProvider) peekSizes() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int(nil), f.peeked...)
}

func (f *fakeGridProvider) awaitPreload(t *testing.T) gridPreloadCall {
	t.Helper()

	select {
	case call := <-f.calls:
		return call
	case <-time.After(2 * time.Second):
		t.Fatal("no preload was dispatched")
		return gridPreloadCall{}
	}
}

type refreshRecorder struct {
	mu  sync.Mutex
	ids []int
}

func (r *refreshRecorder) refresh(id int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ids = append(r.ids, id)
}

func (r *refreshRecorder) seen() []int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]int(nil), r.ids...)
}

func gridTestPath(i int) string {
	return fmt.Sprintf("/photos/DSC%03d.JPG", i)
}

func gridTestPhotos(count int) ([]model.Photo, []model.PhotoMeta) {
	photos := make([]model.Photo, count)
	meta := make([]model.PhotoMeta, count)
	for i := range count {
		path := gridTestPath(i)
		photos[i] = model.Photo{ImagePath: path, Name: filepath.Base(path)}
		meta[i] = model.PhotoMeta{Thumbnail: image.NewNRGBA(image.Rect(0, 0, 2, 2))}
	}
	return photos, meta
}

// The Fyne test driver keeps global canvas and font state, so these must not
// run in parallel.
func TestGridViewerUpdateItemShowsTheCachedImage(t *testing.T) {
	test.NewTempApp(t)

	fake := newFakeGridProvider()
	cached := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	fake.sized = cached
	gv := NewGridViewer(fake, GridViewerCallbacks{})
	photos, meta := gridTestPhotos(1)
	gv.SetPhotos(photos, meta)

	item := newGridItem(200)
	gv.updateItem(0, item)

	assert.Equal(t, "DSC000.JPG", item.name.Text)
	assert.Equal(t, cached, item.thumb.Image)
	// the grid renders tiles of its own on SetPhotos, so what matters is the
	// size every peek asks for and not how many of them there were
	assert.NotEmpty(t, fake.peekSizes())
	for _, size := range fake.peekSizes() {
		assert.Equal(t, gv.thumbPixelSize(), size)
	}
	assert.Empty(t, fake.calls, "a cache hit must not schedule a preload")
}

func TestGridViewerUpdateItemFallsBackToTheEmbeddedThumbnail(t *testing.T) {
	test.NewTempApp(t)

	fake := newFakeGridProvider()
	gv := NewGridViewer(fake, GridViewerCallbacks{})
	photos, meta := gridTestPhotos(1)
	gv.SetPhotos(photos, meta)

	item := newGridItem(200)
	gv.updateItem(0, item)

	assert.Equal(t, meta[0].Thumbnail, item.thumb.Image)

	call := fake.awaitPreload(t)
	assert.Equal(t, gv.thumbPixelSize(), call.size)
	assert.Equal(t, []string{"/photos/DSC000.JPG"}, call.paths)
}

func TestGridViewerUpdateItemIgnoresAnIDPastTheEnd(t *testing.T) {
	test.NewTempApp(t)

	fake := newFakeGridProvider()
	gv := NewGridViewer(fake, GridViewerCallbacks{})
	photos, meta := gridTestPhotos(1)
	gv.SetPhotos(photos, meta)

	item := newGridItem(200)
	peeked := fake.peekSizes()
	gv.updateItem(7, item)

	assert.Equal(t, "placeholder", item.name.Text, "the tile was left untouched")
	assert.Nil(t, item.thumb.Image)
	assert.Equal(t, peeked, fake.peekSizes())
}

func TestGridViewerStopLoadingBumpsTheGeneration(t *testing.T) {
	test.NewTempApp(t)

	fake := newFakeGridProvider()
	gv := NewGridViewer(fake, GridViewerCallbacks{})

	gv.StopLoading()

	assert.Equal(t, uint64(1), fake.Gen())
}

func newRecordingGridViewer(t *testing.T, fake *fakeGridProvider, count int) (*GridViewer, *refreshRecorder) {
	t.Helper()

	gv := NewGridViewer(fake, GridViewerCallbacks{})
	gv.refreshDelay = 10 * time.Millisecond
	refreshes := &refreshRecorder{}
	gv.refreshItem = refreshes.refresh
	photos, meta := gridTestPhotos(count)
	gv.SetPhotos(photos, meta)
	return gv, refreshes
}

func TestGridViewerRefreshesLoadedPhotosOnlyOnce(t *testing.T) {
	test.NewTempApp(t)

	fake := newFakeGridProvider()
	gv, refreshes := newRecordingGridViewer(t, fake, 3)

	gv.updateItem(1, newGridItem(200))
	call := fake.awaitPreload(t)
	require.Len(t, call.paths, 3)

	for _, path := range call.paths {
		call.onLoaded(path)
	}

	require.Eventually(t, func() bool {
		return len(refreshes.seen()) == 1
	}, 2*time.Second, 5*time.Millisecond)
	assert.Never(t, func() bool {
		return len(refreshes.seen()) > 1
	}, 100*time.Millisecond, 10*time.Millisecond, "one refresh already re-runs every visible tile")
}

func TestGridViewerIgnoresLoadsItCannotPlace(t *testing.T) {
	test.NewTempApp(t)

	fake := newFakeGridProvider()
	gv, refreshes := newRecordingGridViewer(t, fake, 3)

	gv.updateItem(1, newGridItem(200))
	call := fake.awaitPreload(t)
	require.Len(t, call.paths, 3)

	call.onLoaded("/photos/GONE.JPG")
	fake.BumpGen()
	call.onLoaded("/photos/DSC000.JPG")

	assert.Never(t, func() bool {
		return len(refreshes.seen()) > 0
	}, 100*time.Millisecond, 10*time.Millisecond)
}

// The grid renders tiles of its own the moment SetPhotos refreshes it, and
// those count as seen just like scrolled ones; a test that wants to know what a
// scroll warms has to start from the state a dispatch leaves behind.
func settleGrid(t *testing.T, gv *GridViewer, fake *fakeGridProvider) {
	t.Helper()

	require.Eventually(t, func() bool {
		return !gv.preloadScheduled.Load()
	}, 2*time.Second, 5*time.Millisecond)

	for {
		select {
		case <-fake.calls:
		default:
			gv.mu.Lock()
			gv.visible.reset()
			gv.mu.Unlock()
			return
		}
	}
}

func TestGridViewerPreloadsAroundTheTileBeingShown(t *testing.T) {
	tests := []struct {
		name      string
		id        int
		count     int
		wantFirst int
		wantLast  int
	}{
		{name: "around the tile that missed", id: 500, count: 1000, wantFirst: 480, wantLast: 520},
		{name: "clamped at the start", id: 1, count: 1000, wantFirst: 0, wantLast: 21},
		{name: "clamped at the end", id: 999, count: 1000, wantFirst: 979, wantLast: 999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			test.NewTempApp(t)

			fake := newFakeGridProvider()
			gv := NewGridViewer(fake, GridViewerCallbacks{})
			photos, meta := gridTestPhotos(tt.count)
			gv.SetPhotos(photos, meta)
			settleGrid(t, gv, fake)

			gv.updateItem(tt.id, newGridItem(200))

			call := fake.awaitPreload(t)
			require.Len(t, call.paths, tt.wantLast-tt.wantFirst+1)
			assert.Equal(t, gridTestPath(tt.wantFirst), call.paths[0])
			assert.Equal(t, gridTestPath(tt.wantLast), call.paths[len(call.paths)-1])
		})
	}
}

func TestGridViewerDispatchesOnePreloadPerMissedTile(t *testing.T) {
	test.NewTempApp(t)

	fake := newFakeGridProvider()
	gv := NewGridViewer(fake, GridViewerCallbacks{})
	photos, meta := gridTestPhotos(1000)
	gv.SetPhotos(photos, meta)
	settleGrid(t, gv, fake)

	gv.updateItem(500, newGridItem(200))
	fake.awaitPreload(t)

	assert.Never(t, func() bool {
		return len(fake.calls) > 0
	}, 200*time.Millisecond, 10*time.Millisecond, "the dispatch must not re-arm itself on its own")
}

func TestGridViewerRearmsAPreloadDroppedWhileOneWasInFlight(t *testing.T) {
	test.NewTempApp(t)

	fake := newFakeGridProvider()
	gv := NewGridViewer(fake, GridViewerCallbacks{})
	photos, meta := gridTestPhotos(1000)
	gv.SetPhotos(photos, meta)
	settleGrid(t, gv, fake)

	release := make(chan struct{})
	fake.blockPreloads(release)

	gv.updateItem(500, newGridItem(200))
	first := fake.awaitPreload(t)
	require.Equal(t, gridTestPath(480), first.paths[0])

	// the tile scrolled to while the dispatch above was still running
	gv.updateItem(900, newGridItem(200))
	close(release)

	second := fake.awaitPreload(t)
	assert.Equal(t, gridTestPath(880), second.paths[0])
	assert.Equal(t, gridTestPath(920), second.paths[len(second.paths)-1])
}
