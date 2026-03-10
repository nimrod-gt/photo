package service

import (
	"sync"

	"photo/model"
)

type Navigator struct {
	mu      sync.Mutex
	photos  []model.Photo
	current int
}

func NewNavigator() *Navigator {
	return &Navigator{
		current: -1,
	}
}

func (n *Navigator) SetPhotos(photos []model.Photo) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.photos = photos
	if len(photos) > 0 {
		n.current = 0
	} else {
		n.current = -1
	}
}

func (n *Navigator) Current() (model.Photo, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.currentLocked()
}

func (n *Navigator) Count() int {
	n.mu.Lock()
	defer n.mu.Unlock()

	return len(n.photos)
}

func (n *Navigator) Next() (model.Photo, int, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.current < len(n.photos)-1 {
		n.current++
	}
	photo, ok := n.currentLocked()
	return photo, n.current, ok
}

func (n *Navigator) Previous() (model.Photo, int, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.current > 0 {
		n.current--
	}
	photo, ok := n.currentLocked()
	return photo, n.current, ok
}

func (n *Navigator) GoTo(index int) (model.Photo, int, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if index >= 0 && index < len(n.photos) {
		n.current = index
	}
	photo, ok := n.currentLocked()
	return photo, n.current, ok
}

func (n *Navigator) Photos() []model.Photo {
	n.mu.Lock()
	defer n.mu.Unlock()

	result := make([]model.Photo, len(n.photos))
	copy(result, n.photos)
	return result
}

func (n *Navigator) Peek(offset int) (model.Photo, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	idx := n.current + offset
	if idx < 0 || idx >= len(n.photos) {
		return model.Photo{}, false
	}
	return n.photos[idx], true
}

func (n *Navigator) CurrentIndex() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.current
}

func (n *Navigator) FindIndex(imagePath string) int {
	n.mu.Lock()
	defer n.mu.Unlock()

	for i, p := range n.photos {
		if p.ImagePath == imagePath {
			return i
		}
	}
	return -1
}

func (n *Navigator) RemoveCurrent() (model.Photo, int, []model.Photo, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.current < 0 || n.current >= len(n.photos) {
		return model.Photo{}, -1, nil, false
	}

	n.photos = append(n.photos[:n.current], n.photos[n.current+1:]...)

	if len(n.photos) == 0 {
		n.current = -1
		return model.Photo{}, -1, nil, false
	}

	if n.current >= len(n.photos) {
		n.current = len(n.photos) - 1
	}

	photos := make([]model.Photo, len(n.photos))
	copy(photos, n.photos)
	photo, ok := n.currentLocked()
	return photo, n.current, photos, ok
}

func (n *Navigator) currentLocked() (model.Photo, bool) {
	if n.current < 0 || n.current >= len(n.photos) {
		return model.Photo{}, false
	}
	return n.photos[n.current], true
}
