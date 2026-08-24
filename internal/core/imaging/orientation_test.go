package imaging

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"photo/internal/core/model"
)

func makeTestImage(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8(x),
				G: uint8(y),
				B: uint8(x + y),
				A: 255,
			})
		}
	}
	return img
}

func pixelAt(img image.Image, x, y int) color.NRGBA {
	r, g, b, a := img.At(x, y).RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}

func TestFlipHorizontal(t *testing.T) {
	t.Parallel()

	src := makeTestImage(4, 3)
	dst := applyOrientation(src, 2)

	require.Equal(t, 4, dst.Bounds().Dx())
	require.Equal(t, 3, dst.Bounds().Dy())

	for y := range 3 {
		for x := range 4 {
			expected := pixelAt(src, 3-x, y)
			actual := pixelAt(dst, x, y)
			assert.Equal(t, expected, actual, "pixel (%d,%d)", x, y)
		}
	}
}

func TestFlipVertical(t *testing.T) {
	t.Parallel()

	src := makeTestImage(4, 3)
	dst := applyOrientation(src, 4)

	require.Equal(t, 4, dst.Bounds().Dx())
	require.Equal(t, 3, dst.Bounds().Dy())

	for y := range 3 {
		for x := range 4 {
			expected := pixelAt(src, x, 2-y)
			actual := pixelAt(dst, x, y)
			assert.Equal(t, expected, actual, "pixel (%d,%d)", x, y)
		}
	}
}

func TestRotate180(t *testing.T) {
	t.Parallel()

	src := makeTestImage(4, 3)
	dst := applyOrientation(src, 3)

	require.Equal(t, 4, dst.Bounds().Dx())
	require.Equal(t, 3, dst.Bounds().Dy())

	for y := range 3 {
		for x := range 4 {
			expected := pixelAt(src, 3-x, 2-y)
			actual := pixelAt(dst, x, y)
			assert.Equal(t, expected, actual, "pixel (%d,%d)", x, y)
		}
	}
}

func TestRotate90CW(t *testing.T) {
	t.Parallel()

	src := makeTestImage(4, 3)
	dst := applyOrientation(src, 6)

	require.Equal(t, 3, dst.Bounds().Dx())
	require.Equal(t, 4, dst.Bounds().Dy())

	for y := range 4 {
		for x := range 3 {
			expected := pixelAt(src, y, 2-x)
			actual := pixelAt(dst, x, y)
			assert.Equal(t, expected, actual, "pixel (%d,%d)", x, y)
		}
	}
}

func TestRotate90CCW(t *testing.T) {
	t.Parallel()

	src := makeTestImage(4, 3)
	dst := applyOrientation(src, 8)

	require.Equal(t, 3, dst.Bounds().Dx())
	require.Equal(t, 4, dst.Bounds().Dy())

	for y := range 4 {
		for x := range 3 {
			expected := pixelAt(src, 3-y, x)
			actual := pixelAt(dst, x, y)
			assert.Equal(t, expected, actual, "pixel (%d,%d)", x, y)
		}
	}
}

func TestTranspose(t *testing.T) {
	t.Parallel()

	src := makeTestImage(4, 3)
	dst := applyOrientation(src, 5)

	require.Equal(t, 3, dst.Bounds().Dx())
	require.Equal(t, 4, dst.Bounds().Dy())

	for y := range 4 {
		for x := range 3 {
			expected := pixelAt(src, y, x)
			actual := pixelAt(dst, x, y)
			assert.Equal(t, expected, actual, "pixel (%d,%d)", x, y)
		}
	}
}

func TestTransverse(t *testing.T) {
	t.Parallel()

	src := makeTestImage(4, 3)
	dst := applyOrientation(src, 7)

	require.Equal(t, 3, dst.Bounds().Dx())
	require.Equal(t, 4, dst.Bounds().Dy())

	for y := range 4 {
		for x := range 3 {
			expected := pixelAt(src, 3-y, 2-x)
			actual := pixelAt(dst, x, y)
			assert.Equal(t, expected, actual, "pixel (%d,%d)", x, y)
		}
	}
}

func TestApplyOrientation(t *testing.T) {
	t.Parallel()

	src := makeTestImage(4, 3)

	tests := []struct {
		orientation int
		wantW       int
		wantH       int
	}{
		{1, 4, 3},
		{2, 4, 3},
		{3, 4, 3},
		{4, 4, 3},
		{5, 3, 4},
		{6, 3, 4},
		{7, 3, 4},
		{8, 3, 4},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("orientation_%d", tt.orientation), func(t *testing.T) {
			dst := applyOrientation(src, tt.orientation)
			assert.Equal(t, tt.wantW, dst.Bounds().Dx())
			assert.Equal(t, tt.wantH, dst.Bounds().Dy())
		})
	}
}

func TestDownscaleToFit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		srcW, srcH  int
		maxW, maxH  int
		wantW       int
		wantH       int
		wantSameRef bool
	}{
		{
			name: "already fits",
			srcW: 100, srcH: 80,
			maxW: 200, maxH: 200,
			wantW: 100, wantH: 80,
			wantSameRef: true,
		},
		{
			name: "exact match",
			srcW: 200, srcH: 200,
			maxW: 200, maxH: 200,
			wantW: 200, wantH: 200,
			wantSameRef: true,
		},
		{
			name: "landscape limited by width",
			srcW: 400, srcH: 200,
			maxW: 200, maxH: 200,
			wantW: 200, wantH: 100,
		},
		{
			name: "portrait limited by height",
			srcW: 200, srcH: 400,
			maxW: 200, maxH: 200,
			wantW: 100, wantH: 200,
		},
		{
			name: "both dimensions exceed",
			srcW: 800, srcH: 600,
			maxW: 400, maxH: 200,
			wantW: 266, wantH: 200,
		},
		{
			name: "zero maxSize skips downscaling",
			srcW: 400, srcH: 200,
			maxW: 0, maxH: 0,
			wantW: 400, wantH: 200,
			wantSameRef: true,
		},
		{
			name: "extreme aspect ratio keeps a non-empty short side",
			srcW: 20000, srcH: 100,
			maxW: 160, maxH: 160,
			wantW: 160, wantH: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := makeTestImage(tt.srcW, tt.srcH)
			dst := DownscaleToFit(src, image.Point{X: tt.maxW, Y: tt.maxH})
			assert.Equal(t, tt.wantW, dst.Bounds().Dx())
			assert.Equal(t, tt.wantH, dst.Bounds().Dy())
			if tt.wantSameRef {
				assert.Same(t, src, dst)
			}
		})
	}
}

func TestApplyOrientationIdentity(t *testing.T) {
	t.Parallel()

	src := makeTestImage(4, 3)
	dst := applyOrientation(src, 1)
	assert.Equal(t, src, dst)
}

func TestToRGBA_AlreadyRGBA(t *testing.T) {
	t.Parallel()

	src := image.NewRGBA(image.Rect(0, 0, 4, 3))
	dst := toRGBA(src)
	assert.Same(t, src, dst)
}

func TestToRGBA_ConvertsOtherTypes(t *testing.T) {
	t.Parallel()

	src := image.NewNRGBA(image.Rect(0, 0, 4, 3))
	src.SetNRGBA(1, 2, color.NRGBA{R: 10, G: 20, B: 30, A: 255})

	dst := toRGBA(src)
	assert.Equal(t, 4, dst.Bounds().Dx())
	assert.Equal(t, 3, dst.Bounds().Dy())
	assert.Equal(t, color.NRGBA{R: 10, G: 20, B: 30, A: 255}, pixelAt(dst, 1, 2))
}

func writeTestJPEG(t *testing.T, w, h int) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, makeTestImage(w, h), nil))
	path := filepath.Join(t.TempDir(), "test.jpg")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0600))
	return path
}

func TestLoadImageOriented(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	plain := writeTestJPEG(t, 4, 2)
	rotated := writeJPEGSizedWithTags(t, dir, "rotated.jpg", 4, 2, map[string]any{
		"Orientation": []uint16{6},
	})

	tests := []struct {
		name  string
		path  string
		wantW int
		wantH int
	}{
		{"EXIF orientation 6 rotates", rotated, 2, 4},
		{"no EXIF keeps dimensions", plain, 4, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, err := LoadImageOriented(tt.path, 0)
			require.NoError(t, err)
			assert.Equal(t, tt.wantW, img.Bounds().Dx())
			assert.Equal(t, tt.wantH, img.Bounds().Dy())
		})
	}

	t.Run("missing file", func(t *testing.T) {
		_, err := LoadImageOriented("/nonexistent/x.jpg", 0)
		assert.Error(t, err)
	})

	// routing on the magic bytes rather than the extension keeps a mislabelled
	// file loading the way image.Decode used to load it
	t.Run("PNG behind a .jpg extension", func(t *testing.T) {
		path := filepath.Join(dir, "actually-png.jpg")
		var buf bytes.Buffer
		require.NoError(t, png.Encode(&buf, makeTestImage(4, 2)))
		require.NoError(t, os.WriteFile(path, buf.Bytes(), 0600))

		img, err := LoadImageOriented(path, 0)
		require.NoError(t, err)
		assert.Equal(t, 4, img.Bounds().Dx())
		assert.Equal(t, 2, img.Bounds().Dy())
	})
}

func TestApplyOrientationSubImage(t *testing.T) {
	t.Parallel()

	full := image.NewRGBA(image.Rect(0, 0, 8, 6))
	for y := range 6 {
		for x := range 8 {
			full.SetRGBA(x, y, color.RGBA{R: uint8(x * 10), G: uint8(y * 10), B: uint8(x + y), A: 255})
		}
	}
	src := full.SubImage(image.Rect(2, 1, 6, 4)).(*image.RGBA)

	dst := applyOrientation(src, 6)
	require.Equal(t, 3, dst.Bounds().Dx())
	require.Equal(t, 4, dst.Bounds().Dy())

	for y := range 4 {
		for x := range 3 {
			expected := pixelAt(src, 2+y, 1+2-x)
			assert.Equal(t, expected, pixelAt(dst, x, y), "pixel (%d,%d)", x, y)
		}
	}
}

func TestLoadImageOrientedDownscales(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		orientation uint16
		fit         int
		wantW       int
		wantH       int
	}{
		{"upright", 1, 20, 20, 10},
		{"rotated", 6, 20, 10, 20},
		{"rotated the other way", 8, 20, 10, 20},
		{"budget larger than source", 6, 400, 20, 40},
	}

	dir := t.TempDir()
	paths := map[uint16]string{}
	for _, orientation := range []uint16{1, 6, 8} {
		paths[orientation] = writeJPEGSizedWithTags(t, dir,
			fmt.Sprintf("orientation-%d.jpg", orientation), 40, 20, map[string]any{
				"Orientation": []uint16{orientation},
			})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, err := LoadImageOriented(paths[tt.orientation], tt.fit)
			require.NoError(t, err)
			assert.Equal(t, tt.wantW, img.Bounds().Dx())
			assert.Equal(t, tt.wantH, img.Bounds().Dy())
		})
	}
}

// the whole point of the RGBA destination is the type-specialized scaler in
// x/image; falling back to NRGBA would stay correct and quietly lose it
func TestDownscaleToFitReturnsRGBA(t *testing.T) {
	t.Parallel()

	dst := DownscaleToFit(makeTestImage(400, 200), image.Point{X: 200, Y: 200})
	assert.IsType(t, &image.RGBA{}, dst)
}

// nothing decodes to *image.YCbCr any more, but the rotation still has to cope
// with a source it cannot index directly
func TestApplyOrientationNonRGBASource(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(writeTestJPEG(t, 8, 6))
	require.NoError(t, err)
	src, err := jpeg.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	require.IsType(t, &image.YCbCr{}, src)

	dst := applyOrientation(src, 6)
	require.IsType(t, &image.RGBA{}, dst)
	require.Equal(t, 6, dst.Bounds().Dx())
	require.Equal(t, 8, dst.Bounds().Dy())

	for y := range 8 {
		for x := range 6 {
			assert.Equal(t, pixelAt(src, y, 5-x), pixelAt(dst, x, y), "pixel (%d,%d)", x, y)
		}
	}
}

// jpegn registers the same magic bytes as image/jpeg, so which decoder
// image.Decode hands a JPEG to is decided by package initialisation order: the
// loader has to route JPEGs by itself, and that route is the one that refuses a
// file which is still being copied.
func TestLoadImageOrientedTruncated(t *testing.T) {
	t.Parallel()

	full, err := os.ReadFile(writeTestJPEG(t, 64, 48))
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "copying.jpg")
	require.NoError(t, os.WriteFile(path, full[:len(full)*60/100], 0600))

	img, err := LoadImageOriented(path, 1600)

	require.Error(t, err)
	assert.Nil(t, img)
}

func TestHasEOI(t *testing.T) {
	t.Parallel()

	full, err := os.ReadFile(writeTestJPEG(t, 64, 48))
	require.NoError(t, err)
	require.True(t, hasEOI(full))

	t.Run("a trailer behind the marker is not a truncation", func(t *testing.T) {
		t.Parallel()
		withTrailer := append(append([]byte{}, full...), []byte("SEFT\x01\x02\x03\n")...)
		assert.True(t, hasEOI(withTrailer))
	})

	t.Run("zero padding behind the marker is not a truncation", func(t *testing.T) {
		t.Parallel()
		padded := append(append([]byte{}, full...), make([]byte, 16)...)
		assert.True(t, hasEOI(padded))
	})

	t.Run("a file cut inside the image data has no marker", func(t *testing.T) {
		t.Parallel()
		assert.False(t, hasEOI(full[:len(full)*60/100]))
	})

	t.Run("a marker inside a segment does not count", func(t *testing.T) {
		t.Parallel()
		segment := append([]byte{markerStart, markerAPP1, 0x00, 0x06}, markerStart, markerEOI, 0x00, 0x00)
		data := append(append([]byte{markerStart, markerSOI}, segment...), full[2:len(full)*60/100]...)
		assert.False(t, hasEOI(data))
	})
}

func TestLoadImageWithStock(t *testing.T) {
	t.Parallel()

	tags := model.Tags{Title: "From the packet.", Keywords: []string{"lisbon", "tram"}}
	packet, ok := packetWithTags(sonyPacket(2000), tags)
	require.True(t, ok)

	t.Run("takes the image and the tags out of one read", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacket(t, t.TempDir(), "tagged.jpg", map[string]any{
			"DateTime": "2026:06:14 08:00:00",
		}, packet)

		loaded, err := LoadImageWithStock(path, 1600)

		require.NoError(t, err)
		require.NotNil(t, loaded.Image)
		require.NoError(t, loaded.StockErr)
		assert.Equal(t, tags, loaded.Stock.Tags)
		assert.Equal(t, time.Date(2026, time.June, 14, 8, 0, 0, 0, time.UTC), loaded.Stock.Taken)
	})

	t.Run("reads the same tags as a parse of the file", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacket(t, t.TempDir(), "same.jpg", map[string]any{
			"ImageDescription": "From the EXIF.",
		}, packet)

		loaded, err := LoadImageWithStock(path, 1600)
		require.NoError(t, err)

		fromFile, err := jpegStockInfo(path)
		require.NoError(t, err)
		assert.Equal(t, fromFile.Tags, loaded.Stock.Tags)
	})

	t.Run("a photo whose EXIF has no date is dated by its file", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacket(t, t.TempDir(), "undated.jpg", map[string]any{
			"ImageDescription": "From the EXIF.",
		}, packet)

		loaded, err := LoadImageWithStock(path, 1600)

		require.NoError(t, err)
		require.NoError(t, loaded.StockErr)
		assert.Equal(t, fileCreated(path), loaded.Stock.Taken)
	})

	t.Run("a format that carries no tags loads without any", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "plain.png")
		var buf bytes.Buffer
		require.NoError(t, png.Encode(&buf, makeTestImage(4, 2)))
		require.NoError(t, os.WriteFile(path, buf.Bytes(), 0600))

		loaded, err := LoadImageWithStock(path, 1600)

		require.NoError(t, err)
		require.NotNil(t, loaded.Image)
		require.NoError(t, loaded.StockErr)
		assert.Equal(t, model.Tags{}, loaded.Stock.Tags)
		assert.Equal(t, fileCreated(path), loaded.Stock.Taken)
	})

	t.Run("a packet it cannot parse still yields the image", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacket(t, t.TempDir(), "broken.jpg", map[string]any{
			"ImageDescription": "From the EXIF.",
		}, xmpPacket("<x:xmpmeta><unclosed>", 40))

		loaded, err := LoadImageWithStock(path, 1600)

		require.NoError(t, err, "the photo is shown whatever its metadata says")
		require.NotNil(t, loaded.Image)
		require.Error(t, loaded.StockErr)
		assert.Contains(t, loaded.StockErr.Error(), "XMP")
		assert.Equal(t, "From the EXIF.", loaded.Stock.Tags.Title)
	})

	t.Run("a file it cannot read yields no image", func(t *testing.T) {
		t.Parallel()

		loaded, err := LoadImageWithStock(filepath.Join(t.TempDir(), "missing.jpg"), 1600)

		require.Error(t, err)
		assert.Nil(t, loaded.Image)
		assert.Equal(t, StockInfo{}, loaded.Stock)
	})
}

// Metadata that cannot be read used to travel back as the error of the load
// itself, and the viewer answered a photo with a broken packet with "Failed to
// load image" instead of showing it.
func TestLoadImageOrientedIgnoresTheTagsError(t *testing.T) {
	t.Parallel()

	path := writeJPEGWithPacket(t, t.TempDir(), "broken.jpg", nil, xmpPacket("<x:xmpmeta><unclosed>", 40))

	img, err := LoadImageOriented(path, 1600)

	require.NoError(t, err)
	assert.NotNil(t, img)
}

func TestExifServiceLoadImage(t *testing.T) {
	t.Parallel()

	svc := NewExifService()
	tags := model.Tags{Title: "Read under the lock."}
	packet, ok := packetWithTags(sonyPacket(2000), tags)
	require.True(t, ok)
	path := writeJPEGWithPacket(t, t.TempDir(), "locked.jpg", nil, packet)

	loaded, err := svc.LoadImage(path, 1600)

	require.NoError(t, err)
	require.NotNil(t, loaded.Image)
	require.NoError(t, loaded.StockErr)
	assert.Equal(t, tags, loaded.Stock.Tags)

	_, err = svc.LoadImage(filepath.Join(t.TempDir(), "missing.jpg"), 1600)
	require.Error(t, err)
}
