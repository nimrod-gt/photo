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

// The real provider hands the paths to its workers and returns at once, so the
// fake must not block either - a dispatch that waited here would hide how often
// the grid asks for one.
func (f *fakeGridProvider) Preload(paths []string, size int, onLoaded func(string)) {
	f.calls <- gridPreloadCall{paths: paths, size: size, onLoaded: onLoaded}
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

// The grid coalesces over two delays, and a test that waited them out would be
// racing the machine it runs on: a stall between two tiles of the same pass is
// indistinguishable from a real second pass. The clock collects what the viewer
// arms instead, and the test decides when it goes off.
type manualClock struct {
	mu    sync.Mutex
	armed []func()
}

func (c *manualClock) after(_ time.Duration, run func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.armed = append(c.armed, run)
}

func (c *manualClock) fire() int {
	c.mu.Lock()
	armed := c.armed
	c.armed = nil
	c.mu.Unlock()

	for _, run := range armed {
		run()
	}
	return len(armed)
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
	gv, clock := newTestGridViewer(fake)
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
	assert.Equal(t, 0, clock.fire(), "a cache hit must not schedule a preload")
	assert.Empty(t, fake.calls)
}

func TestGridViewerUpdateItemFallsBackToTheEmbeddedThumbnail(t *testing.T) {
	test.NewTempApp(t)

	fake := newFakeGridProvider()
	gv, clock := newTestGridViewer(fake)
	photos, meta := gridTestPhotos(1)
	gv.SetPhotos(photos, meta)

	item := newGridItem(200)
	gv.updateItem(0, item)

	assert.Equal(t, meta[0].Thumbnail, item.thumb.Image)

	require.Equal(t, 1, clock.fire())
	call := fake.awaitPreload(t)
	assert.Equal(t, gv.thumbPixelSize(), call.size)
	assert.Equal(t, []string{"/photos/DSC000.JPG"}, call.paths)
}

func TestGridViewerUpdateItemIgnoresAnIDPastTheEnd(t *testing.T) {
	test.NewTempApp(t)

	fake := newFakeGridProvider()
	gv, _ := newTestGridViewer(fake)
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
	gv, _ := newTestGridViewer(fake)

	gv.StopLoading()

	assert.Equal(t, uint64(1), fake.Gen())
}

func newTestGridViewer(fake *fakeGridProvider) (*GridViewer, *manualClock) {
	clock := &manualClock{}
	gv := NewGridViewer(fake, GridViewerCallbacks{})
	gv.after = clock.after
	return gv, clock
}

func newRecordingGridViewer(t *testing.T, fake *fakeGridProvider, count int) (*GridViewer, *manualClock, *refreshRecorder) {
	t.Helper()

	gv, clock := newTestGridViewer(fake)
	refreshes := &refreshRecorder{}
	gv.refreshItem = refreshes.refresh
	photos, meta := gridTestPhotos(count)
	gv.SetPhotos(photos, meta)
	return gv, clock, refreshes
}

func TestGridViewerRefreshesLoadedPhotosOnlyOnce(t *testing.T) {
	test.NewTempApp(t)

	fake := newFakeGridProvider()
	gv, clock, refreshes := newRecordingGridViewer(t, fake, 3)

	gv.updateItem(1, newGridItem(200))
	require.Equal(t, 1, clock.fire())
	call := fake.awaitPreload(t)
	require.Len(t, call.paths, 3)

	for _, path := range call.paths {
		call.onLoaded(path)
	}

	require.Equal(t, 1, clock.fire(), "one refresh already re-runs every visible tile")
	// the refresh itself is handed to the Fyne loop, so it lands a moment later
	require.Eventually(t, func() bool {
		return len(refreshes.seen()) == 1
	}, 2*time.Second, 5*time.Millisecond)
	assert.Equal(t, 0, clock.fire())
}

func TestGridViewerIgnoresLoadsItCannotPlace(t *testing.T) {
	test.NewTempApp(t)

	fake := newFakeGridProvider()
	gv, clock, refreshes := newRecordingGridViewer(t, fake, 3)

	gv.updateItem(1, newGridItem(200))
	require.Equal(t, 1, clock.fire())
	call := fake.awaitPreload(t)
	require.Len(t, call.paths, 3)

	call.onLoaded("/photos/GONE.JPG")
	fake.BumpGen()
	call.onLoaded("/photos/DSC000.JPG")

	assert.Equal(t, 0, clock.fire(), "neither load has a tile to refresh")
	assert.Empty(t, refreshes.seen())
}

// SetPhotos renders tiles of its own before it returns, and those count as seen
// just like scrolled ones - they arm a dispatch that warms the top of the
// folder. A test that wants to know what a scroll warms has to let that
// dispatch through first; it clears the visible range on its way out, so what
// follows starts from nothing seen.
func settleGrid(t *testing.T, fake *fakeGridProvider, clock *manualClock) {
	t.Helper()

	require.Equal(t, 1, clock.fire(), "the tiles SetPhotos rendered arm one dispatch")
	fake.awaitPreload(t)
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
			gv, clock := newTestGridViewer(fake)
			photos, meta := gridTestPhotos(tt.count)
			gv.SetPhotos(photos, meta)
			settleGrid(t, fake, clock)

			gv.updateItem(tt.id, newGridItem(200))

			require.Equal(t, 1, clock.fire())
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
	gv, clock := newTestGridViewer(fake)
	photos, meta := gridTestPhotos(1000)
	gv.SetPhotos(photos, meta)
	settleGrid(t, fake, clock)

	gv.updateItem(500, newGridItem(200))
	require.Equal(t, 1, clock.fire())
	fake.awaitPreload(t)

	assert.Equal(t, 0, clock.fire(), "the dispatch must not re-arm itself on its own")
	assert.Empty(t, fake.calls)
}

func TestGridViewerCoalescesTheMissesOfOnePass(t *testing.T) {
	test.NewTempApp(t)

	fake := newFakeGridProvider()
	gv, clock := newTestGridViewer(fake)
	photos, meta := gridTestPhotos(1000)
	gv.SetPhotos(photos, meta)
	settleGrid(t, fake, clock)

	// a row of tiles drawn in one pass, every one of them a miss
	for id := 500; id < 503; id++ {
		gv.updateItem(id, newGridItem(200))
	}

	require.Equal(t, 1, clock.fire(), "one dispatch per pass, not one per tile")
	call := fake.awaitPreload(t)
	assert.Equal(t, gridTestPath(480), call.paths[0], "the window covers the whole row, not just the first tile")
	assert.Equal(t, gridTestPath(522), call.paths[len(call.paths)-1])
	assert.Empty(t, fake.calls)
}

func TestGridViewerDispatchesAgainForALaterMiss(t *testing.T) {
	test.NewTempApp(t)

	fake := newFakeGridProvider()
	gv, clock := newTestGridViewer(fake)
	photos, meta := gridTestPhotos(1000)
	gv.SetPhotos(photos, meta)
	settleGrid(t, fake, clock)

	gv.updateItem(500, newGridItem(200))
	require.Equal(t, 1, clock.fire())
	first := fake.awaitPreload(t)
	require.Equal(t, gridTestPath(480), first.paths[0])

	gv.updateItem(900, newGridItem(200))

	require.Equal(t, 1, clock.fire())
	second := fake.awaitPreload(t)
	assert.Equal(t, gridTestPath(880), second.paths[0])
	assert.Equal(t, gridTestPath(920), second.paths[len(second.paths)-1])
}
