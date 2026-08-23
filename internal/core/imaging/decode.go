package imaging

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log"

	"github.com/gen2brain/jpegn"
)

var eoiMarker = []byte{markerStart, markerEOI}

// the decoder applies the EXIF orientation itself, so the file's own tag wins
// over anything the caller believes; the returned image is already rotated and
// the caller keeps using the same fit on both axes.
func decodeJPEG(data []byte, fit int, source string) (image.Image, error) {
	return decodeRecovered(func() (image.Image, error) {
		denom, cmyk := 1, false
		if cfg, err := jpegn.DecodeConfig(bytes.NewReader(data)); err == nil {
			denom = scaleDenom(cfg.Width, cfg.Height, fit)
			cmyk = cfg.ColorModel == color.CMYKModel
		}

		// jpegn fills its RGBA buffer for one- and three-component frames only,
		// so asking a four-component one for RGBA - or for a rotation, which
		// implies RGBA - hands back a blank image and no error at all. It has a
		// native path for the CMYK and YCCK frames an Adobe marker describes,
		// but it is reached by so few files that nothing here would keep it
		// honest, and a colour that is merely wrong arrives without an error to
		// show for it.
		if cmyk {
			return decodeStdlib(data, fit, source, "the frame has four components")
		}
		// jpegn decodes a frame that stops early down to wherever it stops and
		// reports no error at all, so a photo still being copied off a card
		// would be kept as a half-grey image until the folder is reloaded.
		if !hasEOI(data) {
			return decodeStdlib(data, fit, source, "the file has no EOI marker behind its image data")
		}

		img, err := jpegn.Decode(bytes.NewReader(data), &jpegn.Options{
			ToRGBA:         true,
			UpsampleMethod: jpegn.CatmullRom,
			AutoRotate:     true,
			ScaleDenom:     denom,
		})
		// jpegn refuses frames of its own - above its pixel cap, or with more
		// pixels than it trusts the bytes to hold - that the standard library
		// decodes, so its refusal is not final either.
		if err != nil {
			return decodeStdlib(data, fit, source, err.Error())
		}
		return img, nil
	})
}

// A file whose copy stopped halfway ends wherever it stopped, so the marker is
// looked for behind the start of the image data rather than at the very end of
// the file: Samsung appends its trailer behind the marker, motion photos a
// whole video, and other writers pad with zeros. Inside the entropy-coded data
// 0xFF is always followed by 0x00 or a restart marker, so the pair cannot occur
// there by accident; the segments in front of it are skipped because the EXIF
// thumbnail carries a marker of its own.
func hasEOI(data []byte) bool {
	spans, err := segmentSpans(data)
	if err != nil {
		return false
	}
	imageStart := 2
	if len(spans) != 0 {
		imageStart = spans[len(spans)-1].end
	}
	return bytes.Contains(data[imageStart:], eoiMarker)
}

// The standard library reads every frame jpegn declines and refuses the ones it
// decodes silently, at the price of decoding at full size: it has no scaling of
// its own, so the frame is shrunk to the budget here, before it is rotated, so
// that the rotation works on the small image rather than on the full one. The
// decoder's own pixel format is kept, the scaler reads it directly. Which files
// take this path is worth knowing, because a folder of them loads several times
// slower for no reason the user can see; only the photo on screen and its
// neighbours come through here, so there is one line per photo shown rather
// than one per file scanned.
func decodeStdlib(data []byte, fit int, source, reason string) (image.Image, error) {
	log.Printf("Decoding %s at full size with the standard library: %s", source, reason)
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	img = DownscaleToFit(img, image.Point{X: fit, Y: fit})
	return applyOrientation(img, exifOrientation(data)), nil
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
// the parent IFD0 — so the caller rotates it. It is decoded by the standard
// library: at a hundred and sixty pixels the speed is worth nothing, while
// refusing a clipped one is what lets the caller fall back to the thumbnail in
// the other IFD, and keeping the decoder's own pixel format is what keeps a
// folder of ten thousand thumbnails at the size it used to be.
func decodeJPEGThumbnail(data []byte) (image.Image, error) {
	return decodeRecovered(func() (image.Image, error) {
		return jpeg.Decode(bytes.NewReader(data))
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
