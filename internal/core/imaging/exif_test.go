package imaging

import (
	"bytes"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	exif "github.com/dsoprea/go-exif/v3"
	jpegstructure "github.com/dsoprea/go-jpeg-image-structure/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writePlainJPEG(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, makeTestImage(8, 6), nil))
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0600))
	return path
}

func writeJPEGWithTags(t *testing.T, dir, name string, tags map[string]any) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, makeTestImage(8, 6), nil))

	jmp := jpegstructure.NewJpegMediaParser()
	intfc, err := jmp.ParseBytes(buf.Bytes())
	require.NoError(t, err)
	sl, ok := intfc.(*jpegstructure.SegmentList)
	require.True(t, ok)

	rootIb, err := sl.ConstructExifBuilder()
	require.NoError(t, err)
	ifd0, err := exif.GetOrCreateIbFromRootIb(rootIb, "IFD0")
	require.NoError(t, err)
	for tagName, value := range tags {
		require.NoError(t, ifd0.SetStandardWithName(tagName, value))
	}
	require.NoError(t, sl.SetExif(rootIb))

	var out bytes.Buffer
	require.NoError(t, sl.Write(&out))
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, out.Bytes(), 0600))
	return path
}

func TestExifService_GetPhotoInfo(t *testing.T) {
	t.Parallel()

	svc := NewExifService()

	t.Run("plain JPEG without EXIF", func(t *testing.T) {
		path := writePlainJPEG(t, t.TempDir(), "plain.jpg")

		thumbnail, rating, orientation, err := svc.GetPhotoInfo(path)

		require.NoError(t, err)
		assert.Nil(t, thumbnail)
		assert.Equal(t, uint16(0), rating)
		assert.Equal(t, uint16(1), orientation)
	})

	t.Run("JPEG with orientation", func(t *testing.T) {
		path := writeJPEGWithTags(t, t.TempDir(), "oriented.jpg", map[string]any{
			"Orientation": []uint16{6},
		})

		_, rating, orientation, err := svc.GetPhotoInfo(path)

		require.NoError(t, err)
		assert.Equal(t, uint16(0), rating)
		assert.Equal(t, uint16(6), orientation)
	})

	t.Run("missing file", func(t *testing.T) {
		_, _, _, err := svc.GetPhotoInfo(filepath.Join(t.TempDir(), "missing.jpg"))
		assert.Error(t, err)
	})

	t.Run("invalid JPEG", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "broken.jpg")
		require.NoError(t, os.WriteFile(path, []byte("not a jpeg"), 0600))

		_, _, _, err := svc.GetPhotoInfo(path)
		assert.Error(t, err)
	})
}

func TestOrientationFromBytes(t *testing.T) {
	t.Parallel()

	t.Run("JPEG with orientation tag", func(t *testing.T) {
		dir := t.TempDir()
		path := writeJPEGWithTags(t, dir, "oriented.jpg", map[string]any{
			"Orientation": []uint16{8},
		})
		data, err := os.ReadFile(path)
		require.NoError(t, err)

		assert.Equal(t, uint16(8), orientationFromBytes(data))
	})

	t.Run("plain JPEG defaults to 1", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, jpeg.Encode(&buf, makeTestImage(8, 6), nil))

		assert.Equal(t, uint16(1), orientationFromBytes(buf.Bytes()))
	})

	t.Run("garbage defaults to 1", func(t *testing.T) {
		assert.Equal(t, uint16(1), orientationFromBytes([]byte("garbage")))
	})
}

func TestExifRootFromFile(t *testing.T) {
	t.Parallel()

	t.Run("no EXIF returns nil without error", func(t *testing.T) {
		path := writePlainJPEG(t, t.TempDir(), "plain.jpg")

		rootIfd, err := exifRootFromFile(path)

		require.NoError(t, err)
		assert.Nil(t, rootIfd)
	})

	t.Run("with EXIF returns root IFD", func(t *testing.T) {
		path := writeJPEGWithTags(t, t.TempDir(), "tagged.jpg", map[string]any{
			"Orientation": []uint16{3},
		})

		rootIfd, err := exifRootFromFile(path)

		require.NoError(t, err)
		require.NotNil(t, rootIfd)
		assert.Equal(t, uint16(3), ifdUint16(rootIfd, "Orientation", 1))
		assert.Equal(t, uint16(7), ifdUint16(rootIfd, "Rating", 7))
	})
}
