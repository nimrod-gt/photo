package imaging

import (
	"bytes"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	exif "github.com/dsoprea/go-exif/v3"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"photo/internal/core/model"
)

func TestExifService_WriteStockTags(t *testing.T) {
	t.Parallel()

	svc := NewExifService()
	written := model.Tags{
		Title:    "A tram climbs the hill in Lisbon.",
		Keywords: []string{"lisbon", "tram", "portugal"},
	}

	t.Run("writes into a JPEG that carries no EXIF", func(t *testing.T) {
		t.Parallel()
		path := writePlainJPEG(t, t.TempDir(), "plain.jpg")

		require.NoError(t, svc.WriteStockTags(path, written))

		info, err := svc.GetStockInfo(model.NewPhoto(path))
		require.NoError(t, err)
		assert.Equal(t, written, info.Tags)
	})

	t.Run("keeps the tags the camera wrote and the image itself", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithTags(t, t.TempDir(), "camera.jpg", map[string]any{
			"Orientation": []uint16{6},
			"Rating":      []uint16{5},
			"Make":        "SONY",
		})

		require.NoError(t, svc.WriteStockTags(path, written))

		_, rating, orientation, err := svc.GetPhotoInfo(path)
		require.NoError(t, err)
		assert.Equal(t, uint16(5), rating)
		assert.Equal(t, uint16(6), orientation)

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = jpeg.Decode(bytes.NewReader(data))
		require.NoError(t, err)
	})

	t.Run("overwrites tags written earlier", func(t *testing.T) {
		t.Parallel()
		path := writePlainJPEG(t, t.TempDir(), "twice.jpg")

		require.NoError(t, svc.WriteStockTags(path, written))
		second := model.Tags{Title: "A quiet morning.", Keywords: []string{"lake", "fog"}}
		require.NoError(t, svc.WriteStockTags(path, second))

		info, err := svc.GetStockInfo(model.NewPhoto(path))
		require.NoError(t, err)
		assert.Equal(t, second, info.Tags)
	})

	t.Run("leaves out what the user did not fill in", func(t *testing.T) {
		t.Parallel()
		path := writePlainJPEG(t, t.TempDir(), "keywords-only.jpg")

		require.NoError(t, svc.WriteStockTags(path, model.Tags{Keywords: []string{"lake"}}))

		info, err := svc.GetStockInfo(model.NewPhoto(path))
		require.NoError(t, err)
		assert.Empty(t, info.Tags.Title)
		assert.Equal(t, []string{"lake"}, info.Tags.Keywords)
	})

	t.Run("keeps the permissions of the original", func(t *testing.T) {
		t.Parallel()
		path := writePlainJPEG(t, t.TempDir(), "perms.jpg")
		require.NoError(t, os.Chmod(path, 0640))

		require.NoError(t, svc.WriteStockTags(path, written))

		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0640), info.Mode().Perm())
	})

	t.Run("keeps what follows the primary image", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := writePlainJPEG(t, dir, "mpf.jpg")
		plain, err := os.ReadFile(path)
		require.NoError(t, err)
		tail := []byte("\xff\xd8 second image appended behind the first one")
		require.NoError(t, os.WriteFile(path, append(plain, tail...), 0600))

		require.NoError(t, svc.WriteStockTags(path, written))

		updated, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.True(t, bytes.HasSuffix(updated, tail), "the appended image must survive the rewrite")
	})

	t.Run("writes into a big endian EXIF block", func(t *testing.T) {
		t.Parallel()
		path := writeBigEndianJPEG(t, t.TempDir(), "motorola.jpg")

		require.NoError(t, svc.WriteStockTags(path, written))

		info, err := svc.GetStockInfo(model.NewPhoto(path))
		require.NoError(t, err)
		assert.Equal(t, written, info.Tags)

		flat, err := flatExifFromFile(path)
		require.NoError(t, err)
		assert.Contains(t, flatValues(flat), "SONY", "the tag that was already there must survive")
	})

	t.Run("keeps the original EXIF bytes and stops growing", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithTags(t, t.TempDir(), "camera.jpg", map[string]any{"Make": "SONY"})
		original := exifBlockOf(t, path)

		require.NoError(t, svc.WriteStockTags(path, written))
		firstSize := sizeOf(t, path)
		assert.True(t, bytes.HasPrefix(exifBlockOf(t, path)[tiffHeaderSize:], original[tiffHeaderSize:]),
			"the camera block must be kept verbatim")

		require.NoError(t, svc.WriteStockTags(path, written))
		require.NoError(t, svc.WriteStockTags(path, written))

		assert.Equal(t, firstSize, sizeOf(t, path), "saving again must reuse the appended IFD")
	})

	t.Run("refuses a file that is not a JPEG", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "notes.txt")
		require.NoError(t, os.WriteFile(path, []byte("not a JPEG at all"), 0600))

		err := svc.WriteStockTags(path, written)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a JPEG")
	})

	t.Run("reports a missing file", func(t *testing.T) {
		t.Parallel()

		err := svc.WriteStockTags(filepath.Join(t.TempDir(), "absent.jpg"), written)

		require.ErrorIs(t, err, os.ErrNotExist)
	})
}

func flatValues(flat []exif.ExifTag) []string {
	values := make([]string, 0, len(flat))
	for _, tag := range flat {
		values = append(values, tag.FormattedFirst)
	}
	return values
}

func exifBlockOf(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	start, end, err := exifSegmentSpan(data)
	require.NoError(t, err)
	require.NotEqual(t, start, end, "the file carries no EXIF")
	return data[start+segmentHeaderSize+len(exifSegmentPrefix) : end]
}

func sizeOf(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.Size()
}

// The Go libraries only ever write little endian EXIF, so a big endian block is
// assembled by hand: header, one IFD with a Make tag, and its value behind it.
func writeBigEndianJPEG(t *testing.T, dir, name string) string {
	t.Helper()
	tiff := []byte{
		'M', 'M', 0, 42, 0, 0, 0, 8,
		0, 1,
		0x01, 0x0f, 0, 2, 0, 0, 0, 5, 0, 0, 0, 26,
		0, 0, 0, 0,
		'S', 'O', 'N', 'Y', 0,
	}
	plain, err := os.ReadFile(writePlainJPEG(t, dir, "plain-source.jpg"))
	require.NoError(t, err)
	withExif, err := spliceExifSegment(plain, 2, 2, tiff)
	require.NoError(t, err)

	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, withExif, 0600))
	return path
}
