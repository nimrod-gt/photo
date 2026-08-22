package imaging

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"

	"github.com/gen2brain/jpegn"
)

// the decoder applies the EXIF orientation itself, so the file's own tag wins
// over anything the caller believes; the returned image is already rotated and
// the caller keeps using the same fit on both axes.
func decodeJPEG(data []byte, fit int) (image.Image, error) {
	return decodeRecovered(func() (image.Image, error) {
		denom, cmyk := 1, false
		if cfg, err := jpegn.DecodeConfig(bytes.NewReader(data)); err == nil {
			denom = scaleDenom(cfg.Width, cfg.Height, fit)
			cmyk = cfg.ColorModel == color.CMYKModel
		}

		if cmyk {
			return decodeCMYK(data, denom)
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
// asking a four-component one for RGBA - or for a rotation, which implies RGBA -
// hands back a blank image and no error at all. Its native path covers the CMYK
// and YCCK frames an Adobe marker describes; anything else, a colour transform
// of 0 among them, still arrives as that blank buffer, and the standard library
// decodes those. Neither path rotates, so the tag is applied here.
func decodeCMYK(data []byte, denom int) (image.Image, error) {
	img, err := jpegn.Decode(bytes.NewReader(data), &jpegn.Options{ScaleDenom: denom})
	if _, blank := img.(*image.RGBA); err != nil || blank {
		if img, err = jpeg.Decode(bytes.NewReader(data)); err != nil {
			return nil, err
		}
	}

	return toRGBA(applyOrientation(img, exifOrientation(data))), nil
}

// DecodeExif reports 0 for an absent tag and passes any out-of-range value
// through, while the parser that actually rotates keeps only 1..8 and defaults
// to 1.
func exifOrientation(data []byte) int {
	if exif, err := jpegn.DecodeExif(bytes.NewReader(data)); err == nil &&
		exif.Orientation >= 1 && exif.Orientation <= 8 {
		return exif.Orientation
	}
	return 1
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
		switch r := recover().(type) {
		case nil:
		case error:
			img, err = nil, fmt.Errorf("decoding jpeg: %w", r)
		default:
			img, err = nil, fmt.Errorf("decoding jpeg: %v", r)
		}
	}()
	return decode()
}

// the largest denominator that still leaves the frame at or above its final
// size, so DownscaleToFit only ever shrinks it the rest of the way. jpegn
// rounds the scaled dimensions up, hence the ceiling division here too.
func scaleDenom(w, h, fit int) int {
	if w <= 0 || h <= 0 || fit <= 0 {
		return 1
	}

	// compared against the final size rather than the budget directly: a
	// panoramic frame has a short side below the budget, which would otherwise
	// reject a denominator that is in fact fine
	outW, outH := fitSize(w, h, image.Point{X: fit, Y: fit})
	for _, d := range []int{8, 4, 2} {
		if (w+d-1)/d >= outW && (h+d-1)/d >= outH {
			return d
		}
	}
	return 1
}
