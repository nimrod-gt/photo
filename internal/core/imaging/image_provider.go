package imaging

import (
	"image"
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"photo/internal/core/model"
)

const maxThumbnailDim = 160

type thumbEntry struct {
	img     image.Image
	decoded bool
}

type stockWaiter struct {
	info StockInfo
	err  error
	done chan struct{}
	// stale says the photo was saved or deleted while this read ran, so what
	// the read found is older than what the cache already holds.
	stale bool
}

type Provider struct {
	loader     *Loader
	exif       *ExifService
	thumbnails sync.Map
	thumbGen   atomic.Uint64

	readStock     func(model.Photo) (StockInfo, error)
	stockMu       sync.Mutex
	stockInflight map[string]*stockWaiter
}

func NewProvider(exif *ExifService) *Provider {
	return &Provider{
		loader:        NewLoader(exif.LoadImage),
		exif:          exif,
		readStock:     exif.GetStockInfo,
		stockInflight: make(map[string]*stockWaiter),
	}
}

// StockInfo answers what the photo says about itself, out of the cache whenever
// the image was loaded, since the tags came in with it. What that read cannot
// know is the XMP sidecar of a RAW pair, which is read once per entry and wins
// over the JPEG exactly as a full read does.
func (p *Provider) StockInfo(photo model.Photo) (StockInfo, error) {
	if info, ok := p.loader.PeekStock(photo.ImagePath); ok && info.complete {
		return info, nil
	}

	p.stockMu.Lock()
	if w, ok := p.stockInflight[photo.ImagePath]; ok {
		p.stockMu.Unlock()
		<-w.done
		return clonedStock(w.info), w.err
	}
	w := &stockWaiter{done: make(chan struct{})}
	p.stockInflight[photo.ImagePath] = w
	p.stockMu.Unlock()

	defer p.removeStockInflight(photo.ImagePath)
	w.info, w.err = p.completeStock(photo)
	close(w.done)
	return w.info, w.err
}

func (p *Provider) completeStock(photo model.Photo) (StockInfo, error) {
	info, cached := p.loader.PeekStock(photo.ImagePath)
	if !cached {
		info, err := p.readStock(photo)
		if err != nil {
			return info, err
		}
		return p.storeRead(photo.ImagePath, info), nil
	}

	if photo.HasRAW() {
		sidecar, err := ReadSidecar(model.SidecarPath(photo.RAWPath))
		if err != nil {
			return info, err
		}
		info.Tags = fillMissing(sidecar, info.Tags)
	}
	return p.storeRead(photo.ImagePath, info), nil
}

// A read is kept unless a save or a delete overtook it: the file it parsed is
// then not the last word on the photo any more, and storing what it found would
// bring back tags that were replaced or a photo that is gone. An overtaken read
// answers with what overtook it, so the caller that asked - and everyone waiting
// on the same read - is told the same thing the cache holds.
func (p *Provider) storeRead(path string, info StockInfo) StockInfo {
	info.complete = true

	p.stockMu.Lock()
	defer p.stockMu.Unlock()
	if w, ok := p.stockInflight[path]; ok && w.stale {
		if newer, ok := p.loader.PeekStock(path); ok {
			return newer
		}
		return info
	}
	p.loader.StoreStock(path, info)
	return info
}

func (p *Provider) markStockStale(paths ...string) {
	for _, path := range paths {
		if w, ok := p.stockInflight[path]; ok {
			w.stale = true
		}
	}
}

func (p *Provider) removeStockInflight(path string) {
	p.stockMu.Lock()
	defer p.stockMu.Unlock()
	delete(p.stockInflight, path)
}

// PeekStockInfo reads nothing: it answers only for a photo whose tags are
// already whole, so a caller on the UI goroutine never waits for a file.
func (p *Provider) PeekStockInfo(path string) (StockInfo, bool) {
	info, ok := p.loader.PeekStock(path)
	if !ok || !info.complete {
		return StockInfo{}, false
	}
	return info, true
}

// PeekStockDate answers the date alone, which the tags of an entry need not be
// whole for: it is settled when the image is read, and the XMP sidecar that
// completeness waits for carries no date.
func (p *Provider) PeekStockDate(path string) (time.Time, bool) {
	info, ok := p.loader.PeekStock(path)
	if !ok || info.Taken.IsZero() {
		return time.Time{}, false
	}
	return info.Taken, true
}

// StoreStockInfo takes what the app itself wrote or generated. It is the whole
// truth for that photo, sidecar included, so nothing is merged into it later.
func (p *Provider) StoreStockInfo(path string, info StockInfo) {
	info.complete = true

	p.stockMu.Lock()
	defer p.stockMu.Unlock()
	p.markStockStale(path)
	p.loader.StoreStock(path, info)
}

func (p *Provider) Forget(paths ...string) {
	p.stockMu.Lock()
	p.markStockStale(paths...)
	p.stockMu.Unlock()

	for _, path := range paths {
		p.loader.Forget(path)
		p.thumbnails.Delete(path)
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
