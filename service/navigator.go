package service

import (
	"photo/model"
)

type Navigator struct {
	photos  []model.Photo
	current int
}

func NewNavigator() *Navigator {
	return &Navigator{
		current: -1,
	}
}

func (n *Navigator) SetPhotos(photos []model.Photo) {
	n.photos = photos
	if len(photos) > 0 {
		n.current = 0
	} else {
		n.current = -1
	}
}

func (n *Navigator) Current() (model.Photo, bool) {
	if n.current < 0 || n.current >= len(n.photos) {
		return model.Photo{}, false
	}
	return n.photos[n.current], true
}

func (n *Navigator) CurrentIndex() int {
	return n.current
}

func (n *Navigator) Count() int {
	return len(n.photos)
}

func (n *Navigator) Next() (model.Photo, bool) {
	if n.current < len(n.photos)-1 {
		n.current++
	}
	return n.Current()
}

func (n *Navigator) Previous() (model.Photo, bool) {
	if n.current > 0 {
		n.current--
	}
	return n.Current()
}

func (n *Navigator) GoTo(index int) (model.Photo, bool) {
	if index >= 0 && index < len(n.photos) {
		n.current = index
	}
	return n.Current()
}

func (n *Navigator) Photos() []model.Photo {
	return n.photos
}

func (n *Navigator) FindIndex(jpegPath string) int {
	for i, p := range n.photos {
		if p.JPEGPath == jpegPath {
			return i
		}
	}
	return -1
}

func (n *Navigator) RemoveCurrent() (model.Photo, bool) {
	if n.current < 0 || n.current >= len(n.photos) {
		return model.Photo{}, false
	}

	n.photos = append(n.photos[:n.current], n.photos[n.current+1:]...)

	if len(n.photos) == 0 {
		n.current = -1
		return model.Photo{}, false
	}

	if n.current >= len(n.photos) {
		n.current = len(n.photos) - 1
	}

	return n.Current()
}
