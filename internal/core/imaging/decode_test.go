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

func TestDecodeJPEGCMYKIsNeverBlank(t *testing.T) {
	t.Parallel()

	data, err := base64.StdEncoding.DecodeString(cmykJPEGBase64)
	require.NoError(t, err)

	cfg, err := jpegn.DecodeConfig(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, color.CMYKModel, cfg.ColorModel)

	img, err := decodeJPEG(data, image.Point{X: 1600, Y: 1600})
	if err != nil {
		assert.Nil(t, img)
		return
	}

	rgba, ok := img.(*image.RGBA)
	require.True(t, ok, "got %T", img)
	assert.NotEqual(t, 0, countNonZero(rgba.Pix), "decoded to an all-zero buffer")
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
