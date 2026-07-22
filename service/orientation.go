package service

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"
)

func LoadOrientedImage(path string) (image.Image, error) {
	return LoadImageOriented(path, 0)
}

// orientation 0 means unknown: sniff it from the JPEG's EXIF data
func LoadImageOriented(path string, orientation uint16) (image.Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading image %s: %w", path, err)
	}

	if orientation == 0 {
		orientation = 1
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".jpg" || ext == ".jpeg" {
			orientation = orientationFromBytes(data)
		}
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decoding image %s: %w", path, err)
	}

	return applyOrientation(img, orientation), nil
}

func orientationFromBytes(data []byte) uint16 {
	rootIfd, err := exifRootFromBytes(data)
	if err != nil {
		log.Printf("Failed to parse JPEG structure for orientation: %v", err)
		return 1
	}
	if rootIfd == nil {
		return 1
	}
	return ifdUint16(rootIfd, "Orientation", 1)
}

func applyOrientation(img image.Image, orientation uint16) image.Image {
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

func toNRGBA(img image.Image) *image.NRGBA {
	if n, ok := img.(*image.NRGBA); ok {
		return n
	}
	b := img.Bounds()
	dst := image.NewNRGBA(b)
	draw.Draw(dst, b, img, b.Min, draw.Src)
	return dst
}

// srcXY maps a destination pixel (x, y) to its source coordinates given the
// source width and height
func transformPixels(img image.Image, swapDims bool, srcXY func(x, y, w, h int) (int, int)) *image.NRGBA {
	src := toNRGBA(img)
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if swapDims {
		dw, dh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			sx, sy := srcXY(x, y, w, h)
			srcIdx := sy*src.Stride + sx*4
			dstIdx := y*dst.Stride + x*4
			copy(dst.Pix[dstIdx:dstIdx+4], src.Pix[srcIdx:srcIdx+4])
		}
	}
	return dst
}

func DownscaleToFit(img image.Image, maxSize image.Point) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxSize.X && h <= maxSize.Y {
		return img
	}

	scaleX := float64(maxSize.X) / float64(w)
	scaleY := float64(maxSize.Y) / float64(h)
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}

	newW := int(float64(w) * scale)
	newH := int(float64(h) * scale)
	dst := image.NewNRGBA(image.Rect(0, 0, newW, newH))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, b, xdraw.Over, nil)
	return dst
}
