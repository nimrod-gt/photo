package imaging

import (
	"bytes"
	"fmt"
	"image"

	"github.com/gen2brain/jpegn"
)

// the decoder applies the EXIF orientation itself, so the file's own tag wins
// over anything the caller believes; the returned image is already rotated and
// the caller keeps using the unswapped maxSize.
func decodeJPEG(data []byte, maxSize image.Point) (img image.Image, err error) {
	// doLoad runs on worker goroutines, where a panic on a single corrupt file
	// would take the whole app down instead of surfacing as a load error
	defer func() {
		if r := recover(); r != nil {
			img, err = nil, fmt.Errorf("decoding jpeg: %v", r)
		}
	}()

	// the budget is expressed in the axes of the rotated result while the
	// denominator is computed in raster axes, so the swap has to read the
	// orientation from the very parser that decides whether to rotate
	budget := maxSize
	if exif, exifErr := jpegn.DecodeExif(bytes.NewReader(data)); exifErr == nil && swapsDimensions(exif.Orientation) {
		budget.X, budget.Y = budget.Y, budget.X
	}

	denom := 1
	if cfg, cfgErr := jpegn.DecodeConfig(bytes.NewReader(data)); cfgErr == nil {
		denom = scaleDenom(cfg.Width, cfg.Height, budget)
	}

	return jpegn.Decode(bytes.NewReader(data), &jpegn.Options{
		ToRGBA:         true,
		UpsampleMethod: jpegn.CatmullRom,
		AutoRotate:     true,
		ScaleDenom:     denom,
	})
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
