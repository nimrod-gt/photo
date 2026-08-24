package imaging

import (
	"image"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru/v2"
)

const (
	cacheMaxEntries = 512
	cacheByteBudget = 1 << 30
)

type cachedImage struct {
	img  image.Image
	size int
	// stock is what the file said about itself when it was read; nil when its
	// tags could not be parsed, so that a later read may still answer them.
	stock *StockInfo
}

type loadWaiter struct {
	img  image.Image
	size int
	err  error
	done chan struct{}
}

type LoadFunc func(path string, size int) (LoadedImage, error)

type Loader struct {
	cache      *lru.Cache[string, cachedImage]
	cacheMu    sync.Mutex
	cacheBytes atomic.Int64
	byteBudget int64
	mu         sync.Mutex
	inflight   map[string]*loadWaiter
	gen        atomic.Uint64
	sem        chan struct{}

	loadImage LoadFunc
}

func NewLoader(load LoadFunc) *Loader {
	workers := max(runtime.NumCPU()-2, 1)

	l := &Loader{
		byteBudget: cacheByteBudget,
		inflight:   make(map[string]*loadWaiter),
		sem:        make(chan struct{}, workers),
		loadImage:  load,
	}
	l.cache = must(lru.NewWithEvict[string, cachedImage](cacheMaxEntries, func(_ string, entry cachedImage) {
		l.cacheBytes.Add(-int64(imageBytes(entry.img)))
	}))
	return l
}

// cacheMu keeps the Peek/Add/evict sequence consistent for budget
// enforcement; cacheBytes itself is atomic so the onEvict callback stays
// correct even if a future code path evicts outside this lock
func (l *Loader) addToCache(path string, entry cachedImage) {
	l.cacheMu.Lock()
	defer l.cacheMu.Unlock()

	if old, ok := l.cache.Peek(path); ok {
		entry.stock = keptStock(entry.stock, old.stock)
	}
	l.storeLocked(path, entry)
}

// Every Add goes through here so the byte budget cannot be sidestepped: a store
// that leaves the image alone still passes an entry whose image may differ from
// the one it replaces, and the accounting has to hold either way.
func (l *Loader) storeLocked(path string, entry cachedImage) {
	// golang-lru v2 does not fire onEvict when Add replaces an existing key
	if old, ok := l.cache.Peek(path); ok {
		l.cacheBytes.Add(-int64(imageBytes(old.img)))
	}
	l.cache.Add(path, entry)
	l.cacheBytes.Add(int64(imageBytes(entry.img)))
	for l.cacheBytes.Load() > l.byteBudget && l.cache.Len() > 1 {
		l.cache.RemoveOldest()
	}
}

// A reload asks for a bigger image, not for the tags again, and tags an entry
// holds that no file carries - generated and not saved yet - live nowhere else,
// so whatever the new read did not find is kept from the entry it replaces.
// Tags already whole outrank the read instead: they carry the XMP sidecar and
// what the app itself wrote, and the JPEG alone would speak over both.
func keptStock(fresh, old *StockInfo) *StockInfo {
	if old == nil {
		return fresh
	}
	if fresh == nil {
		return old
	}
	kept, filler := *fresh, *old
	if old.complete {
		kept, filler = *old, *fresh
	}
	kept.Tags = fillMissing(kept.Tags, filler.Tags)
	if kept.Taken.IsZero() {
		kept.Taken = filler.Taken
	}
	kept.complete = fresh.complete || old.complete
	return &kept
}

func imageBytes(img image.Image) int {
	if img == nil {
		return 0
	}
	switch v := img.(type) {
	case *image.NRGBA:
		return len(v.Pix)
	case *image.RGBA:
		return len(v.Pix)
	case *image.YCbCr:
		// nothing decoded for the cache is YCbCr today, but the default below
		// bills every format at four bytes a pixel, which would bill a
		// subsampled one for two and a half times what it holds
		return len(v.Y) + len(v.Cb) + len(v.Cr)
	default:
		b := img.Bounds()
		return b.Dx() * b.Dy() * 4
	}
}

func (l *Loader) Get(path string, size int) (image.Image, error) {
	size = clampLoadSize(size)

	for {
		loadSize := size
		if entry, ok := l.cache.Get(path); ok {
			if entry.size >= size {
				return entry.img, nil
			}
			loadSize = max(size, entry.size*3/2)
		}

		cached, w, claimed := l.claimInflight(path, size)
		if claimed {
			return l.loadAsOwner(path, loadSize, w)
		}
		if cached != nil {
			return cached, nil
		}
		<-w.done
		if w.err == nil && w.size >= size {
			return w.img, nil
		}
	}
}

// Three outcomes: a cached image big enough to answer with, another
// goroutine's waiter to wait on, or an owned claim. A successful owner fills
// the cache before it leaves the inflight map, so the cache re-check under the
// same lock closes the gap between a cache miss and the claim; a failed or
// superseded owner leaves nothing behind, which is why waiters check w.err and
// w.size instead of trusting the wait alone.
func (l *Loader) claimInflight(path string, size int) (image.Image, *loadWaiter, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if w, ok := l.inflight[path]; ok {
		return nil, w, false
	}
	if entry, ok := l.cache.Get(path); ok && entry.size >= size {
		return entry.img, nil, false
	}
	w := &loadWaiter{done: make(chan struct{})}
	l.inflight[path] = w
	return nil, w, true
}

func (l *Loader) runLoad(path string, size int, w *loadWaiter) (LoadedImage, error) {
	loaded, err := l.loadImage(path, size)
	w.img = loaded.Image
	w.size = size
	w.err = err
	close(w.done)
	return loaded, err
}

func (l *Loader) cacheLoaded(path string, size int, loaded LoadedImage, err error) {
	if err != nil {
		return
	}
	l.addToCache(path, cachedImage{img: loaded.Image, size: size, stock: stockOf(loaded)})
}

func (l *Loader) loadAsOwner(path string, loadSize int, w *loadWaiter) (image.Image, error) {
	defer l.removeInflight(path)
	loaded, err := l.runLoad(path, loadSize, w)
	l.cacheLoaded(path, loadSize, loaded, err)
	return loaded.Image, err
}

// Tags that could not be read are left out of the entry rather than cached as
// empty ones: the photo is still shown, and the next reader may do better.
func stockOf(loaded LoadedImage) *StockInfo {
	if loaded.StockErr != nil {
		return nil
	}
	stock := loaded.Stock
	return &stock
}

func (l *Loader) Peek(path string, size int) image.Image {
	size = clampLoadSize(size)

	if entry, ok := l.cache.Peek(path); ok && entry.size >= size {
		return entry.img
	}
	return nil
}

func (l *Loader) PeekStock(path string) (StockInfo, bool) {
	entry, ok := l.cache.Peek(path)
	if !ok || entry.stock == nil {
		return StockInfo{}, false
	}
	return clonedStock(*entry.stock), true
}

// The entry owns its keywords: a caller that stored or read tags may go on
// editing its own slice, and appending to one shared with the cache would
// change what the next reader gets.
func clonedStock(info StockInfo) StockInfo {
	info.Tags.Keywords = slices.Clone(info.Tags.Keywords)
	return info
}

// An entry without an image holds tags alone - the ones generated or saved for
// a photo that is not in the cache. Its size is zero, which no Get or Peek can
// ask for, so it is never handed out as an image.
// The date the photo was taken is read from the file and never written, so a
// store that carries none keeps the one the entry already learned instead of
// erasing it: nothing would read the file for it a second time.
func (l *Loader) StoreStock(path string, info StockInfo) {
	l.cacheMu.Lock()
	defer l.cacheMu.Unlock()

	entry, ok := l.cache.Peek(path)
	if !ok {
		entry = cachedImage{}
	}
	stock := clonedStock(info)
	if stock.Taken.IsZero() && entry.stock != nil {
		stock.Taken = entry.stock.Taken
	}
	entry.stock = &stock
	l.storeLocked(path, entry)
}

func (l *Loader) Forget(path string) {
	l.cacheMu.Lock()
	defer l.cacheMu.Unlock()

	l.cache.Remove(path)
}

func (l *Loader) Preload(paths []string, size int, onLoaded func(string)) {
	size = clampLoadSize(size)
	gen := l.gen.Load()

	for _, p := range paths {
		if gen != l.gen.Load() {
			return
		}

		if entry, ok := l.cache.Peek(p); ok && entry.size >= size {
			continue
		}

		_, w, claimed := l.claimInflight(p, size)
		if !claimed {
			continue
		}

		path := p
		go func() {
			l.sem <- struct{}{}
			defer func() { <-l.sem }()
			defer l.removeInflight(path)

			if gen != l.gen.Load() {
				close(w.done)
				return
			}

			loaded, err := l.runLoad(path, size, w)

			if gen != l.gen.Load() {
				return
			}

			l.cacheLoaded(path, size, loaded, err)
			if onLoaded != nil && err == nil {
				onLoaded(path)
			}
		}()
	}
}

func (l *Loader) Gen() uint64 {
	return l.gen.Load()
}

func (l *Loader) BumpGen() {
	l.gen.Add(1)
}

func (l *Loader) removeInflight(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.inflight, key)
}

func (l *Loader) Clear() {
	l.gen.Add(1)
	l.cacheMu.Lock()
	l.cache.Purge()
	l.cacheMu.Unlock()

	l.mu.Lock()
	l.inflight = make(map[string]*loadWaiter)
	l.mu.Unlock()
}

// DownscaleToFit reads a non-positive budget as "no downscaling", which
// would cache every photo at full resolution and blow the byte budget. Clamping
// at the entry points keeps the cached image and the size it is cached under
// from disagreeing.
func clampLoadSize(size int) int {
	return max(size, 1)
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
