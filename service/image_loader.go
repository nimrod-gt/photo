package service

import (
	"image"
	"runtime"
	"sync"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru/v2"
)

type SizeClass int

const (
	SizeThumb SizeClass = iota
	SizeFull

	thumbCacheSize = 1024
	fullCacheSize  = 128
)

type loadWaiter struct {
	img  image.Image
	err  error
	done chan struct{}
	gen  uint64
}

type ImageLoader struct {
	thumbs   *lru.Cache[string, image.Image]
	fulls    *lru.Cache[string, image.Image]
	thumbMax image.Point
	fullMax  image.Point
	exif     *ExifService
	mu       sync.Mutex
	inflight map[string]*loadWaiter
	gen      atomic.Uint64
	sem      chan struct{}

	loadFull  func(string) (image.Image, error)
	loadThumb func(string) (image.Image, error)
}

func NewImageLoader(thumbMax, fullMax image.Point, exif *ExifService) *ImageLoader {
	thumbs, _ := lru.New[string, image.Image](thumbCacheSize)
	fulls, _ := lru.New[string, image.Image](fullCacheSize)
	workers := max(runtime.NumCPU()-1, 2)

	l := &ImageLoader{
		thumbs:   thumbs,
		fulls:    fulls,
		thumbMax: thumbMax,
		fullMax:  fullMax,
		exif:     exif,
		inflight: make(map[string]*loadWaiter),
		sem:      make(chan struct{}, workers),
	}
	l.loadFull = l.doLoadFull
	l.loadThumb = l.doLoadThumb
	return l
}

func (l *ImageLoader) cacheFor(sc SizeClass) *lru.Cache[string, image.Image] {
	if sc == SizeFull {
		return l.fulls
	}
	return l.thumbs
}

func cacheKey(path string, sc SizeClass) string {
	if sc == SizeFull {
		return "F:" + path
	}
	return "T:" + path
}

func (l *ImageLoader) Get(path string, sc SizeClass) (image.Image, error) {
	cache := l.cacheFor(sc)
	if img, ok := cache.Get(path); ok {
		return img, nil
	}

	key := cacheKey(path, sc)

	l.mu.Lock()
	if w, ok := l.inflight[key]; ok {
		l.mu.Unlock()
		<-w.done
		return w.img, w.err
	}

	w := &loadWaiter{done: make(chan struct{}), gen: l.gen.Load()}
	l.inflight[key] = w
	l.mu.Unlock()

	defer l.removeInflight(key)
	w.img, w.err = l.load(path, sc)
	close(w.done)

	if w.err == nil {
		cache.Add(path, w.img)
	}

	return w.img, w.err
}

func (l *ImageLoader) Peek(path string, sc SizeClass) image.Image {
	cache := l.cacheFor(sc)
	if img, ok := cache.Peek(path); ok {
		return img
	}
	return nil
}

func (l *ImageLoader) Preload(paths []string, sc SizeClass, onLoaded func(string)) {
	gen := l.gen.Load()
	cache := l.cacheFor(sc)

	for _, p := range paths {
		if gen != l.gen.Load() {
			return
		}

		if _, ok := cache.Peek(p); ok {
			continue
		}

		key := cacheKey(p, sc)
		l.mu.Lock()
		if _, ok := l.inflight[key]; ok {
			l.mu.Unlock()
			continue
		}

		w := &loadWaiter{done: make(chan struct{}), gen: gen}
		l.inflight[key] = w
		l.mu.Unlock()

		path := p
		l.sem <- struct{}{}
		go func() {
			defer func() { <-l.sem }()
			defer l.removeInflight(key)

			if gen != l.gen.Load() {
				close(w.done)
				return
			}

			w.img, w.err = l.load(path, sc)
			close(w.done)

			if gen != l.gen.Load() {
				return
			}

			if w.err == nil {
				cache.Add(path, w.img)
			}

			if onLoaded != nil && w.err == nil {
				onLoaded(path)
			}
		}()
	}
}

func (l *ImageLoader) removeInflight(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.inflight, key)
}

func (l *ImageLoader) Clear() {
	l.gen.Add(1)
	l.thumbs.Purge()
	l.fulls.Purge()

	l.mu.Lock()
	l.inflight = make(map[string]*loadWaiter)
	l.mu.Unlock()
}

func (l *ImageLoader) load(path string, sc SizeClass) (image.Image, error) {
	if sc == SizeFull {
		return l.loadFull(path)
	}
	return l.loadThumb(path)
}

func (l *ImageLoader) doLoadFull(path string) (image.Image, error) {
	img, err := LoadOrientedImage(path)
	if err != nil {
		return nil, err
	}
	return DownscaleToFit(img, l.fullMax), nil
}

func (l *ImageLoader) doLoadThumb(path string) (image.Image, error) {
	if img, ok := l.fulls.Peek(path); ok {
		return DownscaleToFit(img, l.thumbMax), nil
	}

	if l.exif != nil {
		thumb, _, err := l.exif.GetPhotoInfo(path)
		if err == nil && thumb != nil {
			b := thumb.Bounds()
			if b.Dx() >= l.thumbMax.X && b.Dy() >= l.thumbMax.Y {
				return DownscaleToFit(thumb, l.thumbMax), nil
			}
		}
	}

	img, err := LoadOrientedImage(path)
	if err != nil {
		return nil, err
	}
	l.fulls.Add(path, DownscaleToFit(img, l.fullMax))
	return DownscaleToFit(img, l.thumbMax), nil
}
