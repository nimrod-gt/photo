package imaging

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/gen2brain/jpegn"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeJPEGThumbnail(t *testing.T) {
	t.Parallel()

	t.Run("decodes to RGBA", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, jpeg.Encode(&buf, makeTestImage(16, 12), nil))

		img, err := decodeJPEGThumbnail(buf.Bytes())
		require.NoError(t, err)
		assert.IsType(t, &image.RGBA{}, img)
		assert.Equal(t, 16, img.Bounds().Dx())
		assert.Equal(t, 12, img.Bounds().Dy())
	})

	// thumbnail bytes come from a third-party EXIF parser and are decoded on
	// worker goroutines, so a bad one has to surface as an error, not a panic
	t.Run("corrupt data returns an error", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, jpeg.Encode(&buf, makeTestImage(16, 12), nil))
		data := buf.Bytes()
		for i := 4; i < len(data); i += 3 {
			data[i] = 0x5a
		}

		img, err := decodeJPEGThumbnail(data)
		require.Error(t, err)
		assert.Nil(t, img)
	})
}

// an 8x8 four-component JPEG with an Adobe colour transform of 0, written by
// macOS sips. jpegn takes such a frame for RGB and returns a blank RGBA buffer
// with no error, which would put a black photo in the cache and in the grid
const cmykJPEGBase64 = "" +
	"/9j/4AAQSkZJRgABAQAASABIAAD/4QBARXhpZgAATU0AKgAAAAgAAYdpAAQAAAABAAAAGgAAAAAA" +
	"AqACAAQAAAABAAAACKADAAQAAAABAAAACAAAAAD/7QA4UGhvdG9zaG9wIDMuMAA4QklNBAQAAAAA" +
	"AAA4QklNBCUAAAAAABDUHYzZjwCyBOmACZjs+EJ+/+4ADkFkb2JlAGQAAAAAAP/AABQIAAgACAQB" +
	"EQACEQEDEQEEEQH/xAAfAAABBQEBAQEBAQAAAAAAAAAAAQIDBAUGBwgJCgv/xAC1EAACAQMDAgQD" +
	"BQUEBAAAAX0BAgMABBEFEiExQQYTUWEHInEUMoGRoQgjQrHBFVLR8CQzYnKCCQoWFxgZGiUmJygp" +
	"KjQ1Njc4OTpDREVGR0hJSlNUVVZXWFlaY2RlZmdoaWpzdHV2d3h5eoOEhYaHiImKkpOUlZaXmJma" +
	"oqOkpaanqKmqsrO0tba3uLm6wsPExcbHyMnK0tPU1dbX2Nna4eLj5OXm5+jp6vHy8/T19vf4+fr/" +
	"xAAfAQADAQEBAQEBAQEBAAAAAAAAAQIDBAUGBwgJCgv/xAC1EQACAQIEBAMEBwUEBAABAncAAQID" +
	"EQQFITEGEkFRB2FxEyIygQgUQpGhscEJIzNS8BVictEKFiQ04SXxFxgZGiYnKCkqNTY3ODk6Q0RF" +
	"RkdISUpTVFVWV1hZWmNkZWZnaGlqc3R1dnd4eXqCg4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqy" +
	"s7S1tre4ubrCw8TFxsfIycrS09TV1tfY2dri4+Tl5ufo6ery8/T19vf4+fr/2wBDAAICAgICAgMC" +
	"AgMFAwMDBQYFBQUFBggGBgYGBggKCAgICAgICgoKCgoKCgoMDAwMDAwODg4ODg8PDw8PDw8PDw//" +
	"2wBDAQIDAwQEBAcEBAcQCwkLEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQ" +
	"EBAQEBAQEBAQEBD/3QAEAAH/2gAOBAEAAhEDEQQRAD8A4f8AYd/5KLc/9d/610fRm/5HVT/EfY/R" +
	"u/5HVX/Ef6CfRE/5NvgfT9T/2Q=="

func TestDecodeJPEGCMYK(t *testing.T) {
	t.Parallel()

	data, err := base64.StdEncoding.DecodeString(cmykJPEGBase64)
	require.NoError(t, err)

	cfg, err := jpegn.DecodeConfig(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, color.CMYKModel, cfg.ColorModel)

	img, err := decodeJPEG(data, 1600)
	require.NoError(t, err)

	rgba, ok := img.(*image.RGBA)
	require.True(t, ok, "got %T", img)
	require.Equal(t, image.Rect(0, 0, 8, 8), rgba.Bounds())
	assert.NotEqual(t, 0, countNonZero(rgba.Pix), "decoded to an all-zero buffer")

	want, err := jpeg.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	for y := range 8 {
		for x := range 8 {
			assert.Equal(t, color.RGBAModel.Convert(want.At(x, y)), rgba.At(x, y), "pixel (%d,%d)", x, y)
		}
	}
}

func countNonZero(pix []byte) int {
	n := 0
	for _, b := range pix {
		if b != 0 {
			n++
		}
	}
	return n
}

func TestScaleDenom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		w, h int
		fit  int
		want int
	}{
		{"24MP frame at a viewer-sized budget", 6000, 4000, 1728, 2},
		{"the same frame where halving would undershoot", 6000, 4000, 3456, 1},
		{"61MP frame reaches a quarter", 9504, 6336, 1728, 4},
		// the short side of a panorama is already inside the budget, so a
		// denominator judged against the budget rather than against the final
		// size would be rejected for no reason
		{"panorama", 6000, 1000, 1728, 2},
		{"zero budget means no downscaling", 6000, 4000, 0, 1},
		{"negative budget means no downscaling", 6000, 4000, -1, 1},
		{"budget larger than the frame", 100, 100, 1728, 1},
		{"degenerate frame", 0, 0, 1728, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, scaleDenom(tt.w, tt.h, tt.fit))
		})
	}
}

// the denominator is only safe while the decoded frame stays at or above its
// final size: anything smaller would be stretched back up by Fyne
func TestScaleDenomNeverUndershoots(t *testing.T) {
	t.Parallel()

	for _, fit := range []int{160, 1117, 1728, 3456} {
		for _, frame := range []image.Point{{X: 6000, Y: 4000}, {X: 4000, Y: 6000}, {X: 9504, Y: 6336}, {X: 6000, Y: 1000}, {X: 1919, Y: 1081}} {
			d := scaleDenom(frame.X, frame.Y, fit)
			outW, outH := fitSize(frame.X, frame.Y, image.Point{X: fit, Y: fit})
			assert.GreaterOrEqual(t, (frame.X+d-1)/d, outW, "frame %v budget %d denom %d", frame, fit, d)
			assert.GreaterOrEqual(t, (frame.Y+d-1)/d, outH, "frame %v budget %d denom %d", frame, fit, d)
		}
	}
}

func TestDecodeJPEGAppliesScaleDenom(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, makeTestImage(400, 300), nil))
	data := buf.Bytes()

	tests := []struct {
		name  string
		fit   int
		wantW int
		wantH int
	}{
		{"eighth", 50, 50, 38},
		{"half", 110, 200, 150},
		{"no budget", 0, 400, 300},
		{"budget larger than the frame", 800, 400, 300},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, err := decodeJPEG(data, tt.fit)
			require.NoError(t, err)
			// a YCbCr result would put the generic scaler back in the pipeline
			assert.IsType(t, &image.RGBA{}, img)
			assert.Equal(t, tt.wantW, img.Bounds().Dx())
			assert.Equal(t, tt.wantH, img.Bounds().Dy())
		})
	}
}

// the decoders differ in how they upsample chroma, so this pins that they agree
// on the picture, not on the bits. On a camera JPEG single pixels along sharp
// edges drift by around 27 while the mean stays under 1
func TestDecodeJPEGMatchesStdlib(t *testing.T) {
	t.Parallel()

	const w, h = 64, 48
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, makeTestImage(w, h), nil))
	data := buf.Bytes()

	got, err := decodeJPEG(data, 0)
	require.NoError(t, err)
	want, err := jpeg.Decode(bytes.NewReader(data))
	require.NoError(t, err)

	require.Equal(t, want.Bounds().Dx(), got.Bounds().Dx())
	require.Equal(t, want.Bounds().Dy(), got.Bounds().Dy())

	total := 0
	for y := range h {
		for x := range w {
			gr, gg, gb, _ := got.At(x, y).RGBA()
			wr, wg, wb, _ := want.At(x, y).RGBA()
			for _, d := range []int{
				channelDelta(gr, wr), channelDelta(gg, wg), channelDelta(gb, wb),
			} {
				assert.LessOrEqual(t, d, 12, "pixel (%d,%d)", x, y)
				total += d
			}
		}
	}
	assert.Less(t, float64(total)/float64(w*h*3), 1.0, "mean channel delta")
}

func channelDelta(a, b uint32) int {
	d := int(a>>8) - int(b>>8)
	if d < 0 {
		return -d
	}
	return d
}

// a corrupt file must surface as a load error rather than take down the worker
// goroutine it decodes on
func TestDecodeJPEGCorrupt(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, makeTestImage(64, 48), nil))
	full := buf.Bytes()

	tests := []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"magic bytes only", []byte{0xff, 0xd8}},
		{"header cut in half", full[:len(full)/8]},
		{"not a JPEG at all", []byte("certainly not a jpeg")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, err := decodeJPEG(tt.data, 1600)
			require.Error(t, err)
			assert.Nil(t, img)
		})
	}
}
