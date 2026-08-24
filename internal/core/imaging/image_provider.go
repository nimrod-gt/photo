package imaging

import (
	"image"
	"log"
	"runtime"
	"sync"
	"sync/atomic"

	"photo/internal/core/model"
)

const maxThumbnailDim = 160

type thumbEntry struct {
	img     image.Image
	decoded bool
}

type Provider struct {
	loader     *Loader
	exif       *ExifService
	thumbnails sync.Map
	thumbGen   atomic.Uint64
}

func NewProvider(exif *ExifService) *Provider {
	return &Provider{
		loader: NewLoader(exif.LoadImage),
		exif:   exif,
	}
}

func (p *Provider) Get(path string, size int) (image.Image, error) {
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

func (p *Provider) Peek(path string, size int) image.Image {
	if thumb := p.peekThumbnail(path, size); thumb != nil {
		return thumb
	}
	return p.loader.Peek(path, size)
}

func (p *Provider) peekThumbnail(path string, size int) image.Image {
	if size > maxThumbnailDim {
		return nil
	}
	v, ok := p.thumbnails.Load(path)
	if !ok {
		return nil
	}
	img := v.(thumbEntry).img
	b := img.Bounds()
	if b.Dx() >= size || b.Dy() >= size {
		return img
	}
	return nil
}

func (p *Provider) Preload(paths []string, size int, onLoaded func(string)) {
	p.loader.Preload(paths, size, func(path string) {
		if img := p.loader.Peek(path, size); img != nil {
			p.storeThumbnail(path, img)
		}
		if onLoaded != nil {
			onLoaded(path)
		}
	})
}

func (p *Provider) Thumbnail(path string) image.Image {
	if v, ok := p.thumbnails.Load(path); ok {
		return v.(thumbEntry).img
	}
	return nil
}

// LoadedMeta is what the folder scan learns about one JPEG from its own bytes.
type LoadedMeta struct {
	Thumbnail image.Image
	Favorite  bool
	Ratable   bool
}

func (p *Provider) LoadFolder(
	photos []model.Photo,
	onLoaded func(index int, meta LoadedMeta),
	onComplete func(),
) {
	gen := p.thumbGen.Add(1)
	p.loader.Clear()
	p.thumbnails.Clear()

	go func() {
		workers := max(runtime.NumCPU()-2, 1)
		sem := make(chan struct{}, workers)
		var wg sync.WaitGroup
		for i, photo := range photos {
			if p.thumbGen.Load() != gen {
				break
			}
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				if p.thumbGen.Load() != gen {
					return
				}
				if !photo.IsJPEG() {
					onLoaded(i, LoadedMeta{})
					return
				}
				info, err := p.exif.GetPhotoInfo(photo.ImagePath)
				if err != nil {
					log.Printf("Failed to read EXIF for %s: %v", photo.Name, err)
					onLoaded(i, LoadedMeta{})
					return
				}
				if p.thumbGen.Load() != gen {
					return
				}
				if info.Thumbnail != nil {
					p.thumbnails.LoadOrStore(photo.ImagePath, thumbEntry{img: info.Thumbnail})
				}
				onLoaded(i, LoadedMeta{Thumbnail: info.Thumbnail, Favorite: info.Rating > 0, Ratable: info.Ratable})
			}()
		}
		wg.Wait()
		if p.thumbGen.Load() == gen && onComplete != nil {
			onComplete()
		}
	}()
}

func (p *Provider) Clear() {
	p.thumbGen.Add(1)
	p.loader.Clear()
	p.thumbnails.Clear()
}

func (p *Provider) BumpGen() {
	p.loader.BumpGen()
}

func (p *Provider) Gen() uint64 {
	return p.loader.Gen()
}

func (p *Provider) storeThumbnail(path string, fullImg image.Image) {
	if v, ok := p.thumbnails.Load(path); ok && v.(thumbEntry).decoded {
		return
	}
	maxSize := image.Point{X: maxThumbnailDim, Y: maxThumbnailDim}
	thumb := DownscaleToFit(fullImg, maxSize)
	p.thumbnails.Store(path, thumbEntry{img: thumb, decoded: true})
}
