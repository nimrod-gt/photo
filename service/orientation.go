package service

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	jpegstructure "github.com/dsoprea/go-jpeg-image-structure/v2"
	xdraw "golang.org/x/image/draw"
)

func LoadOrientedImage(path string) (image.Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading image %s: %w", path, err)
	}

	var orientation uint16 = 1
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".jpg" || ext == ".jpeg" {
		orientation = orientationFromBytes(data)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decoding image %s: %w", path, err)
	}

	return applyOrientation(img, orientation), nil
}

func orientationFromBytes(data []byte) uint16 {
	jmp := jpegstructure.NewJpegMediaParser()
	intfc, err := jmp.ParseBytes(data)
	if err != nil {
		return 1
	}

	sl, ok := intfc.(*jpegstructure.SegmentList)
	if !ok {
		return 1
	}

	rootIfd, _, err := sl.Exif()
	if err != nil {
		return 1
	}

	results, err := rootIfd.FindTagWithName("Orientation")
	if err != nil || len(results) == 0 {
		return 1
	}

	value, err := results[0].Value()
	if err != nil {
		return 1
	}

	if v, ok := value.([]uint16); ok && len(v) > 0 {
		return v[0]
	}

	return 1
}

func applyOrientation(img image.Image, orientation uint16) image.Image {
	switch orientation {
	case 2:
		return flipHorizontal(img)
	case 3:
		return rotate180(img)
	case 4:
		return flipVertical(img)
	case 5:
		return transpose(img)
	case 6:
		return rotate90CW(img)
	case 7:
		return transverse(img)
	case 8:
		return rotate90CCW(img)
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

func flipHorizontal(img image.Image) *image.NRGBA {
	src := toNRGBA(img)
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			srcIdx := y*src.Stride + (w-1-x)*4
			dstIdx := y*dst.Stride + x*4
			copy(dst.Pix[dstIdx:dstIdx+4], src.Pix[srcIdx:srcIdx+4])
		}
	}
	return dst
}

func flipVertical(img image.Image) *image.NRGBA {
	src := toNRGBA(img)
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		srcRow := (h - 1 - y) * src.Stride
		dstRow := y * dst.Stride
		copy(dst.Pix[dstRow:dstRow+w*4], src.Pix[srcRow:srcRow+w*4])
	}
	return dst
}

func rotate180(img image.Image) *image.NRGBA {
	src := toNRGBA(img)
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			srcIdx := (h-1-y)*src.Stride + (w-1-x)*4
			dstIdx := y*dst.Stride + x*4
			copy(dst.Pix[dstIdx:dstIdx+4], src.Pix[srcIdx:srcIdx+4])
		}
	}
	return dst
}

func rotate90CW(img image.Image) *image.NRGBA {
	src := toNRGBA(img)
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			srcIdx := y*src.Stride + x*4
			dstIdx := x*dst.Stride + (h-1-y)*4
			copy(dst.Pix[dstIdx:dstIdx+4], src.Pix[srcIdx:srcIdx+4])
		}
	}
	return dst
}

func rotate90CCW(img image.Image) *image.NRGBA {
	src := toNRGBA(img)
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			srcIdx := y*src.Stride + x*4
			dstIdx := (w-1-x)*dst.Stride + y*4
			copy(dst.Pix[dstIdx:dstIdx+4], src.Pix[srcIdx:srcIdx+4])
		}
	}
	return dst
}

func transpose(img image.Image) *image.NRGBA {
	src := toNRGBA(img)
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			srcIdx := y*src.Stride + x*4
			dstIdx := x*dst.Stride + y*4
			copy(dst.Pix[dstIdx:dstIdx+4], src.Pix[srcIdx:srcIdx+4])
		}
	}
	return dst
}

func transverse(img image.Image) *image.NRGBA {
	src := toNRGBA(img)
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			srcIdx := y*src.Stride + x*4
			dstIdx := (w-1-x)*dst.Stride + (h-1-y)*4
			copy(dst.Pix[dstIdx:dstIdx+4], src.Pix[srcIdx:srcIdx+4])
		}
	}
	return dst
}

func downscaleToFit(img image.Image, maxSize image.Point) image.Image {
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
