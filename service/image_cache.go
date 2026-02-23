package service

import (
	"image"
	"sync"
)

type cacheEntry struct {
	img  image.Image
	err  error
	done chan struct{}
}

type ImageCache struct {
	mu        sync.Mutex
	items     map[string]*cacheEntry
	loadImage func(string) (image.Image, error)
}

func NewImageCache(maxSize image.Point) *ImageCache {
	c := &ImageCache{
		items: make(map[string]*cacheEntry),
	}
	c.loadImage = func(path string) (image.Image, error) {
		img, err := LoadOrientedImage(path)
		if err != nil {
			return nil, err
		}
		return downscaleToFit(img, maxSize), nil
	}
	return c
}

func (c *ImageCache) Load(path string) (image.Image, error) {
	c.mu.Lock()
	if entry, ok := c.items[path]; ok {
		c.mu.Unlock()
		<-entry.done
		return entry.img, entry.err
	}

	entry := &cacheEntry{done: make(chan struct{})}
	c.items[path] = entry
	c.mu.Unlock()

	entry.img, entry.err = c.loadImage(path)
	close(entry.done)
	return entry.img, entry.err
}

func (c *ImageCache) Prefetch(path string) {
	c.mu.Lock()
	if _, ok := c.items[path]; ok {
		c.mu.Unlock()
		return
	}

	entry := &cacheEntry{done: make(chan struct{})}
	c.items[path] = entry
	c.mu.Unlock()

	go func() {
		defer close(entry.done)
		entry.img, entry.err = c.loadImage(path)
	}()
}

func (c *ImageCache) EvictExcept(keep []string) {
	keepSet := make(map[string]struct{}, len(keep))
	for _, p := range keep {
		keepSet[p] = struct{}{}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for path := range c.items {
		if _, ok := keepSet[path]; !ok {
			delete(c.items, path)
		}
	}
}

func (c *ImageCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*cacheEntry)
}
