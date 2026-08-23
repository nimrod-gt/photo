package imaging

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/png"
	"os"

	xdraw "golang.org/x/image/draw"
)

var jpegMagic = []byte{0xff, 0xd8}

// fit is a budget on both sides of the returned image.
func LoadImageOriented(path string, fit int) (image.Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading image %s: %w", path, err)
	}

	img, err := decodeImage(data, fit, path)
	if err != nil {
		return nil, fmt.Errorf("decoding image %s: %w", path, err)
	}
	return DownscaleToFit(img, image.Point{X: fit, Y: fit}), nil
}

// JPEG is routed by its magic bytes rather than by extension, and its EXIF
// orientation is read and applied by the decoder itself. The other formats the
// viewer opens carry no orientation at all, so nothing rotates them.
func decodeImage(data []byte, fit int, source string) (image.Image, error) {
	if bytes.HasPrefix(data, jpegMagic) {
		return decodeJPEG(data, fit, source)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}

func applyOrientation(img image.Image, orientation int) image.Image {
	switch orientation {
	case 2:
		return transformPixels(img, false, func(x, y, w, _ int) (int, int) { return w - 1 - x, y })
	case 3:
		return transformPixels(img, false, func(x, y, w, h int) (int, int) { return w - 1 - x, h - 1 - y })
	case 4:
		return transformPixels(img, false, func(x, y, _, h int) (int, int) { return x, h - 1 - y })
	case 5:
		return transformPixels(img, true, func(x, y, _, _ int) (int, int) { return y, x })
	case 6:
		return transformPixels(img, true, func(x, y, _, h int) (int, int) { return y, h - 1 - x })
	case 7:
		return transformPixels(img, true, func(x, y, w, h int) (int, int) { return w - 1 - y, h - 1 - x })
	case 8:
		return transformPixels(img, true, func(x, y, w, _ int) (int, int) { return w - 1 - y, x })
	default:
		return img
	}
}

// transformPixels indexes Pix directly, so a source of any other format is
// converted first
func toRGBA(img image.Image) *image.RGBA {
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba
	}
	b := img.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, img, b.Min, draw.Src)
	return dst
}

// srcXY maps a destination pixel (x, y) to its source coordinates given the
// source width and height
func transformPixels(img image.Image, swapDims bool, srcXY func(x, y, w, h int) (int, int)) *image.RGBA {
	src := toRGBA(img)
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if swapDims {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := range dh {
		for x := range dw {
			sx, sy := srcXY(x, y, w, h)
			srcIdx := sy*src.Stride + sx*4
			dstIdx := y*dst.Stride + x*4
			copy(dst.Pix[dstIdx:dstIdx+4], src.Pix[srcIdx:srcIdx+4])
		}
	}
	return dst
}

// a zero or negative maxSize coordinate means "no downscaling"
func DownscaleToFit(img image.Image, maxSize image.Point) image.Image {
	if maxSize.X <= 0 || maxSize.Y <= 0 {
		return img
	}

	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	newW, newH := fitSize(w, h, maxSize)
	if newW == w && newH == h {
		return img
	}

	// the *image.RGBA destination is what buys the speed: any other destination
	// type falls back to a per-pixel generic path. The op is not what matters —
	// Src is used because a fresh buffer has nothing to composite against
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, b, xdraw.Src, nil)
	return dst
}

func fitSize(w, h int, maxSize image.Point) (int, int) {
	if w <= maxSize.X && h <= maxSize.Y {
		return w, h
	}

	scale := min(float64(maxSize.X)/float64(w), float64(maxSize.Y)/float64(h))

	// an extreme aspect ratio truncates the short side to zero, which would
	// cache an empty image
	return max(int(float64(w)*scale), 1), max(int(float64(h)*scale), 1)
}
