package imaging

import (
	"image"
	"runtime"
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
}

type loadWaiter struct {
	img  image.Image
	size int
	err  error
	done chan struct{}
}

type Loader struct {
	cache      *lru.Cache[string, cachedImage]
	cacheMu    sync.Mutex
	cacheBytes atomic.Int64
	byteBudget int64
	mu         sync.Mutex
	inflight   map[string]*loadWaiter
	gen        atomic.Uint64
	sem        chan struct{}

	loadImage func(path string, size int) (image.Image, error)
}

func NewLoader() *Loader {
	workers := max(runtime.NumCPU()-2, 1)

	l := &Loader{
		byteBudget: cacheByteBudget,
		inflight:   make(map[string]*loadWaiter),
		sem:        make(chan struct{}, workers),
		loadImage: func(path string, size int) (image.Image, error) {
			return LoadImageOriented(path, image.Point{X: size, Y: size})
		},
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

func imageBytes(img image.Image) int {
	switch v := img.(type) {
	case *image.NRGBA:
		return len(v.Pix)
	case *image.RGBA:
		return len(v.Pix)
	case *image.YCbCr:
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

		l.mu.Lock()
		if w, ok := l.inflight[path]; ok {
			l.mu.Unlock()
			<-w.done
			if w.err == nil && w.size >= size {
				return w.img, nil
			}
			continue
		}

		if entry, ok := l.cache.Get(path); ok && entry.size >= size {
			l.mu.Unlock()
			return entry.img, nil
		}

		w := &loadWaiter{done: make(chan struct{})}
		l.inflight[path] = w
		l.mu.Unlock()

		return l.loadAsOwner(path, loadSize, w)
	}
}

func (l *Loader) loadAsOwner(path string, loadSize int, w *loadWaiter) (image.Image, error) {
	defer l.removeInflight(path)
	img, err := l.doLoad(path, loadSize)
	w.img = img
	w.size = loadSize
	w.err = err
	close(w.done)

	if err == nil {
		l.addToCache(path, cachedImage{img: img, size: loadSize})
	}

	return img, err
}

func (l *Loader) Peek(path string, size int) image.Image {
	if entry, ok := l.cache.Peek(path); ok && entry.size >= size {
		return entry.img
	}
	return nil
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

		l.mu.Lock()
		if _, ok := l.inflight[p]; ok {
			l.mu.Unlock()
			continue
		}

		w := &loadWaiter{done: make(chan struct{})}
		l.inflight[p] = w
		l.mu.Unlock()

		path := p
		go func() {
			l.sem <- struct{}{}
			defer func() { <-l.sem }()
			defer l.removeInflight(path)

			if gen != l.gen.Load() {
				close(w.done)
				return
			}

			img, err := l.doLoad(path, size)
			w.img = img
			w.size = size
			w.err = err
			close(w.done)

			if gen != l.gen.Load() {
				return
			}

			if err == nil {
				l.addToCache(path, cachedImage{img: img, size: size})
			}

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

// DownscaleToFit reads a non-positive budget as "no downscaling", which would
// cache every photo at full resolution and blow the byte budget. Clamping at
// the entry points rather than inside doLoad keeps the cached image and the
// size it is cached under from disagreeing.
func clampLoadSize(size int) int {
	return max(size, 1)
}

func (l *Loader) doLoad(path string, size int) (image.Image, error) {
	img, err := l.loadImage(path, size)
	if err != nil {
		return nil, err
	}
	// a no-op once loadImage honours size itself; it keeps the Loader contract
	// for alternative loadImage implementations that ignore it
	return DownscaleToFit(img, image.Point{X: size, Y: size}), nil
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
