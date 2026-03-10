package service

import (
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestImage(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
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
	src := makeTestImage(4, 3)
	dst := flipHorizontal(src)

	require.Equal(t, 4, dst.Bounds().Dx())
	require.Equal(t, 3, dst.Bounds().Dy())

	for y := 0; y < 3; y++ {
		for x := 0; x < 4; x++ {
			expected := pixelAt(src, 3-x, y)
			actual := pixelAt(dst, x, y)
			assert.Equal(t, expected, actual, "pixel (%d,%d)", x, y)
		}
	}
}

func TestFlipVertical(t *testing.T) {
	src := makeTestImage(4, 3)
	dst := flipVertical(src)

	require.Equal(t, 4, dst.Bounds().Dx())
	require.Equal(t, 3, dst.Bounds().Dy())

	for y := 0; y < 3; y++ {
		for x := 0; x < 4; x++ {
			expected := pixelAt(src, x, 2-y)
			actual := pixelAt(dst, x, y)
			assert.Equal(t, expected, actual, "pixel (%d,%d)", x, y)
		}
	}
}

func TestRotate180(t *testing.T) {
	src := makeTestImage(4, 3)
	dst := rotate180(src)

	require.Equal(t, 4, dst.Bounds().Dx())
	require.Equal(t, 3, dst.Bounds().Dy())

	for y := 0; y < 3; y++ {
		for x := 0; x < 4; x++ {
			expected := pixelAt(src, 3-x, 2-y)
			actual := pixelAt(dst, x, y)
			assert.Equal(t, expected, actual, "pixel (%d,%d)", x, y)
		}
	}
}

func TestRotate90CW(t *testing.T) {
	src := makeTestImage(4, 3)
	dst := rotate90CW(src)

	require.Equal(t, 3, dst.Bounds().Dx())
	require.Equal(t, 4, dst.Bounds().Dy())

	for y := 0; y < 4; y++ {
		for x := 0; x < 3; x++ {
			expected := pixelAt(src, y, 2-x)
			actual := pixelAt(dst, x, y)
			assert.Equal(t, expected, actual, "pixel (%d,%d)", x, y)
		}
	}
}

func TestRotate90CCW(t *testing.T) {
	src := makeTestImage(4, 3)
	dst := rotate90CCW(src)

	require.Equal(t, 3, dst.Bounds().Dx())
	require.Equal(t, 4, dst.Bounds().Dy())

	for y := 0; y < 4; y++ {
		for x := 0; x < 3; x++ {
			expected := pixelAt(src, 3-y, x)
			actual := pixelAt(dst, x, y)
			assert.Equal(t, expected, actual, "pixel (%d,%d)", x, y)
		}
	}
}

func TestTranspose(t *testing.T) {
	src := makeTestImage(4, 3)
	dst := transpose(src)

	require.Equal(t, 3, dst.Bounds().Dx())
	require.Equal(t, 4, dst.Bounds().Dy())

	for y := 0; y < 4; y++ {
		for x := 0; x < 3; x++ {
			expected := pixelAt(src, y, x)
			actual := pixelAt(dst, x, y)
			assert.Equal(t, expected, actual, "pixel (%d,%d)", x, y)
		}
	}
}

func TestTransverse(t *testing.T) {
	src := makeTestImage(4, 3)
	dst := transverse(src)

	require.Equal(t, 3, dst.Bounds().Dx())
	require.Equal(t, 4, dst.Bounds().Dy())

	for y := 0; y < 4; y++ {
		for x := 0; x < 3; x++ {
			expected := pixelAt(src, 3-y, 2-x)
			actual := pixelAt(dst, x, y)
			assert.Equal(t, expected, actual, "pixel (%d,%d)", x, y)
		}
	}
}

func TestApplyOrientation(t *testing.T) {
	src := makeTestImage(4, 3)

	tests := []struct {
		orientation uint16
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
	src := makeTestImage(4, 3)
	dst := applyOrientation(src, 1)
	assert.Equal(t, src, dst)
}

func TestToNRGBA_AlreadyNRGBA(t *testing.T) {
	src := makeTestImage(4, 3)
	dst := toNRGBA(src)
	assert.Same(t, src, dst)
}

func TestToNRGBA_ConvertsOtherTypes(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 3))
	src.Set(1, 2, color.NRGBA{R: 10, G: 20, B: 30, A: 255})

	dst := toNRGBA(src)
	assert.Equal(t, 4, dst.Bounds().Dx())
	assert.Equal(t, 3, dst.Bounds().Dy())
	assert.Equal(t, color.NRGBA{R: 10, G: 20, B: 30, A: 255}, pixelAt(dst, 1, 2))
}
