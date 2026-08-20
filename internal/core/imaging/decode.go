package imaging

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"

	"github.com/gen2brain/jpegn"
)

var errCMYKUnsupported = errors.New("unsupported CMYK jpeg")

// the decoder applies the EXIF orientation itself, so the file's own tag wins
// over anything the caller believes; the returned image is already rotated and
// the caller keeps using the unswapped maxSize.
func decodeJPEG(data []byte, maxSize image.Point) (image.Image, error) {
	return decodeRecovered(func() (image.Image, error) {
		// DecodeExif reports 0 for an absent tag and passes any out-of-range
		// value through, while the parser that actually rotates keeps only 1..8
		// and defaults to 1. Narrowing to the same range here stops a malformed
		// tag from transposing the budget below while the pixels stay upright.
		orientation := 1
		if exif, err := jpegn.DecodeExif(bytes.NewReader(data)); err == nil &&
			exif.Orientation >= 1 && exif.Orientation <= 8 {
			orientation = exif.Orientation
		}

		// the budget is expressed in the axes of the rotated result while the
		// denominator is computed in raster axes
		budget := maxSize
		if swapsDimensions(orientation) {
			budget.X, budget.Y = budget.Y, budget.X
		}

		denom, cmyk := 1, false
		if cfg, err := jpegn.DecodeConfig(bytes.NewReader(data)); err == nil {
			denom = scaleDenom(cfg.Width, cfg.Height, budget)
			cmyk = cfg.ColorModel == color.CMYKModel
		}

		if cmyk {
			return decodeCMYK(data, orientation, denom)
		}

		return jpegn.Decode(bytes.NewReader(data), &jpegn.Options{
			ToRGBA:         true,
			UpsampleMethod: jpegn.CatmullRom,
			AutoRotate:     true,
			ScaleDenom:     denom,
		})
	})
}

// jpegn fills its RGBA buffer for one- and three-component frames only, so
// asking a four-component one for RGBA — or for a rotation, which implies RGBA —
// hands back a blank image and no error at all. Decoding it natively and turning
// it into RGBA here is the same decoder taking a different exit, not a fallback.
func decodeCMYK(data []byte, orientation, denom int) (image.Image, error) {
	img, err := jpegn.Decode(bytes.NewReader(data), &jpegn.Options{ScaleDenom: denom})
	if err != nil {
		return nil, err
	}

	// an Adobe colour transform of 0 makes jpegn take the frame for RGB even at
	// four components, and it then hands back the blank RGBA buffer instead of
	// an error. A native four-component decode is only ever *image.CMYK, so any
	// RGBA here is that empty buffer
	if _, blank := img.(*image.RGBA); blank {
		return nil, errCMYKUnsupported
	}

	return toRGBA(applyOrientation(img, orientation)), nil
}

// an embedded thumbnail carries no EXIF of its own — the orientation belongs to
// the parent IFD0 — so AutoRotate would find nothing and the caller rotates it.
// It is already at its final size, so there is nothing for ScaleDenom to save.
func decodeJPEGThumbnail(data []byte) (image.Image, error) {
	return decodeRecovered(func() (image.Image, error) {
		return jpegn.Decode(bytes.NewReader(data), &jpegn.Options{
			ToRGBA:         true,
			UpsampleMethod: jpegn.CatmullRom,
		})
	})
}

// decoding runs on worker goroutines, where a panic on a single corrupt file
// would take the whole app down instead of surfacing as a load error
func decodeRecovered(decode func() (image.Image, error)) (img image.Image, err error) {
	defer func() {
		if r := recover(); r != nil {
			img, err = nil, fmt.Errorf("decoding jpeg: %v", r)
		}
	}()
	return decode()
}

// the largest denominator that still leaves the frame at or above its final
// size, so DownscaleToFit only ever shrinks it the rest of the way. jpegn
// rounds the scaled dimensions up, hence the ceiling division here too.
func scaleDenom(w, h int, maxSize image.Point) int {
	if w <= 0 || h <= 0 || maxSize.X <= 0 || maxSize.Y <= 0 {
		return 1
	}

	// compared against the final size rather than the budget directly: a
	// panoramic frame has a short side below the budget, which would otherwise
	// reject a denominator that is in fact fine
	outW, outH := fitSize(w, h, maxSize)
	for _, d := range []int{8, 4, 2} {
		if (w+d-1)/d >= outW && (h+d-1)/d >= outH {
			return d
		}
	}
	return 1
}
