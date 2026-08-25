package ui

import (
	"image"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// Smooth scaling is what Fyne paints an image with by default, and it costs a
// CatmullRom pass over the whole image plus a copy out of the NRGBA it produces,
// on the UI goroutine, every time a tile shows a different photo. ImageFillContain
// makes the widget box differ from the image on one axis, so that never lands on
// the cheap path, while the images cached for these widgets are already about the
// size they are drawn at - which is exactly what the GPU filters well on its own.
func newThumbnailImage(width, height float32) *canvas.Image {
	thumb := canvas.NewImageFromImage(nil)
	thumb.SetMinSize(fyne.NewSize(width, height))
	thumb.FillMode = canvas.ImageFillContain
	thumb.ScaleMode = canvas.ImageScaleFastest
	return thumb
}

type thumbnailSource interface {
	Peek(path string, size int) image.Image
	Thumbnail(path string) image.Image
}

// size <= 0 asks for the small cached thumbnail only and must not go through
// Peek: the loader clamps the size to 1 and would hand back the cached
// full-resolution image, which a 48x36 row must never hold.
func resolveThumbnail(src thumbnailSource, path string, embedded image.Image, size int) (image.Image, bool) {
	var cached image.Image
	if size > 0 {
		cached = src.Peek(path, size)
	} else {
		cached = src.Thumbnail(path)
	}
	if cached != nil {
		return cached, false
	}
	return embedded, true
}

// Fyne refreshes every visible row on any list refresh, and both Image and Label
// refreshes are expensive (texture upload, text shaping), so skip unchanged ones.
func updateThumbnail(thumb *canvas.Image, img image.Image) {
	if thumb.Image == img && thumb.Visible() == (img != nil) {
		return
	}
	if img != nil {
		thumb.Image = img
		thumb.Show()
	} else {
		thumb.Image = nil
		thumb.Hide()
	}
	thumb.Refresh()
}
