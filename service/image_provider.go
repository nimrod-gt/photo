package service

import (
	"image"
	"log"
	"sync"
	"sync/atomic"

	"photo/model"
)

const maxThumbnailDim = 160

type ImageProvider struct {
	loader     *ImageLoader
	exif       *ExifService
	thumbnails sync.Map
	thumbGen   atomic.Uint64
}

func NewImageProvider(exif *ExifService) *ImageProvider {
	return &ImageProvider{
		loader: NewImageLoader(),
		exif:   exif,
	}
}

func (p *ImageProvider) Get(path string, size int) (image.Image, error) {
	if thumb := p.peekThumbnail(path, size); thumb != nil {
		return thumb, nil
	}

	img, err := p.loader.Get(path, size)
	if err != nil {
		return nil, err
	}

	p.storeThumbnail(path, img)
	return img, nil
}

func (p *ImageProvider) Peek(path string, size int) image.Image {
	if thumb := p.peekThumbnail(path, size); thumb != nil {
		return thumb
	}
	return p.loader.Peek(path, size)
}

func (p *ImageProvider) peekThumbnail(path string, size int) image.Image {
	if size > maxThumbnailDim {
		return nil
	}
	v, ok := p.thumbnails.Load(path)
	if !ok {
		return nil
	}
	img := v.(image.Image)
	b := img.Bounds()
	if b.Dx() >= size || b.Dy() >= size {
		return img
	}
	return nil
}

func (p *ImageProvider) Preload(paths []string, size int, onLoaded func(string)) {
	p.loader.Preload(paths, size, func(path string) {
		if img := p.loader.Peek(path, size); img != nil {
			p.storeThumbnail(path, img)
		}
		if onLoaded != nil {
			onLoaded(path)
		}
	})
}

func (p *ImageProvider) Thumbnail(path string) image.Image {
	if thumb, ok := p.thumbnails.Load(path); ok {
		return thumb.(image.Image)
	}
	return nil
}

func (p *ImageProvider) LoadFolder(
	photos []model.Photo,
	onLoaded func(index int, thumbnail image.Image, favorite bool),
	onComplete func(),
) {
	p.loader.Clear()
	p.thumbnails = sync.Map{}
	gen := p.thumbGen.Add(1)

	go func() {
		for i, photo := range photos {
			if p.thumbGen.Load() != gen {
				return
			}
			if !photo.IsJPEG() {
				onLoaded(i, nil, false)
				continue
			}
			thumbnail, rating, err := p.exif.GetPhotoInfo(photo.ImagePath)
			if err != nil {
				log.Printf("Failed to read EXIF for %s: %v", photo.Name, err)
				onLoaded(i, nil, false)
				continue
			}
			if p.thumbGen.Load() != gen {
				return
			}
			if thumbnail != nil {
				p.thumbnails.Store(photo.ImagePath, thumbnail)
			}
			onLoaded(i, thumbnail, rating > 0)
		}
		if p.thumbGen.Load() == gen && onComplete != nil {
			onComplete()
		}
	}()
}

func (p *ImageProvider) Clear() {
	p.loader.Clear()
	p.thumbnails = sync.Map{}
	p.thumbGen.Add(1)
}

func (p *ImageProvider) BumpGen() {
	p.loader.BumpGen()
}

func (p *ImageProvider) Gen() uint64 {
	return p.loader.Gen()
}

func (p *ImageProvider) storeThumbnail(path string, fullImg image.Image) {
	maxSize := image.Point{X: maxThumbnailDim, Y: maxThumbnailDim}
	thumb := DownscaleToFit(fullImg, maxSize)
	p.thumbnails.LoadOrStore(path, thumb)
}
