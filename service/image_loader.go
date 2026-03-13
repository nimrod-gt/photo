package service

import (
	"image"
	"runtime"
	"sync"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru/v2"
)

const cacheSize = 128

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

type ImageLoader struct {
	cache    *lru.Cache[string, cachedImage]
	mu       sync.Mutex
	inflight map[string]*loadWaiter
	gen      atomic.Uint64
	sem      chan struct{}

	loadImage func(string) (image.Image, error)
}

func NewImageLoader() *ImageLoader {
	cache := must(lru.New[string, cachedImage](cacheSize))
	workers := max(runtime.NumCPU()-2, 1)

	return &ImageLoader{
		cache:     cache,
		inflight:  make(map[string]*loadWaiter),
		sem:       make(chan struct{}, workers),
		loadImage: LoadOrientedImage,
	}
}

func (l *ImageLoader) Get(path string, size int) (image.Image, error) {
	if entry, ok := l.cache.Get(path); ok && entry.size >= size {
		return entry.img, nil
	}

	l.mu.Lock()
	if w, ok := l.inflight[path]; ok {
		l.mu.Unlock()
		<-w.done
		if w.err == nil && w.size >= size {
			return w.img, nil
		}
		img, err := l.doLoad(path, size)
		if err == nil {
			l.cache.Add(path, cachedImage{img: img, size: size})
		}
		return img, err
	}

	if entry, ok := l.cache.Get(path); ok && entry.size >= size {
		l.mu.Unlock()
		return entry.img, nil
	}

	w := &loadWaiter{done: make(chan struct{})}
	l.inflight[path] = w
	l.mu.Unlock()

	defer l.removeInflight(path)
	img, err := l.doLoad(path, size)
	w.img = img
	w.size = size
	w.err = err
	close(w.done)

	if err == nil {
		l.cache.Add(path, cachedImage{img: img, size: size})
	}

	return img, err
}

func (l *ImageLoader) Peek(path string, size int) image.Image {
	if entry, ok := l.cache.Peek(path); ok && entry.size >= size {
		return entry.img
	}
	return nil
}

func (l *ImageLoader) Preload(paths []string, size int, onLoaded func(string)) {
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
				l.cache.Add(path, cachedImage{img: img, size: size})
			}

			if onLoaded != nil && err == nil {
				onLoaded(path)
			}
		}()
	}
}

func (l *ImageLoader) Gen() uint64 {
	return l.gen.Load()
}

func (l *ImageLoader) BumpGen() {
	l.gen.Add(1)
}

func (l *ImageLoader) removeInflight(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.inflight, key)
}

func (l *ImageLoader) Clear() {
	l.gen.Add(1)
	l.cache.Purge()

	l.mu.Lock()
	l.inflight = make(map[string]*loadWaiter)
	l.mu.Unlock()
}

func (l *ImageLoader) doLoad(path string, size int) (image.Image, error) {
	img, err := l.loadImage(path)
	if err != nil {
		return nil, err
	}
	return DownscaleToFit(img, image.Point{X: size, Y: size}), nil
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
