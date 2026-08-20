package imaging

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

const (
	benchSrcW = 4000
	benchSrcH = 3000
	benchFit  = 1600
)

// a synthetic gradient compresses far better than a photograph, so decoding it
// under-reports the entropy work that dominates a real load; point
// PHOTO_BENCH_JPEG at a camera JPEG to measure the case the app actually runs
func benchJPEGPath(b *testing.B) string {
	b.Helper()
	if path := os.Getenv("PHOTO_BENCH_JPEG"); len(path) != 0 {
		if _, err := os.Stat(path); err != nil {
			b.Fatal(err)
		}
		return path
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, makeTestImage(benchSrcW, benchSrcH), nil); err != nil {
		b.Fatal(err)
	}
	path := filepath.Join(b.TempDir(), "bench.jpg")
	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		b.Fatal(err)
	}
	return path
}

func BenchmarkDecodeJPEG(b *testing.B) {
	data, err := os.ReadFile(benchJPEGPath(b))
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadImageOriented(b *testing.B) {
	path := benchJPEGPath(b)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := LoadImageOriented(path, 6, image.Point{X: benchFit, Y: benchFit}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDownscaleToFit(b *testing.B) {
	data, err := os.ReadFile(benchJPEGPath(b))
	if err != nil {
		b.Fatal(err)
	}
	src, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		b.Fatal(err)
	}
	// the whole point of the benchmark is the YCbCr fast path in x/image/draw;
	// a different pixel type would silently measure something else
	if _, ok := src.(*image.YCbCr); !ok {
		b.Fatalf("expected *image.YCbCr source, got %T", src)
	}

	b.ReportAllocs()
	for b.Loop() {
		_ = DownscaleToFit(src, image.Point{X: benchFit, Y: benchFit})
	}
}

// orientation 1 is the common case and the only one where the decoded
// *image.YCbCr reaches DownscaleToFit unconverted
func BenchmarkLoadImageOrientedUpright(b *testing.B) {
	path := benchJPEGPath(b)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := LoadImageOriented(path, 1, image.Point{X: benchFit, Y: benchFit}); err != nil {
			b.Fatal(err)
		}
	}
}
