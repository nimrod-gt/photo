package imaging

import (
	"bytes"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	exif "github.com/dsoprea/go-exif/v3"
	jpegstructure "github.com/dsoprea/go-jpeg-image-structure/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"photo/internal/core/model"
)

func writeJPEGWithIfdTags(t *testing.T, dir, name string, byIfd map[string]map[string]any) string {
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
	for ifdPath, tags := range byIfd {
		ib, err := exif.GetOrCreateIbFromRootIb(rootIb, ifdPath)
		require.NoError(t, err)
		for tagName, value := range tags {
			require.NoError(t, ib.SetStandardWithName(tagName, value))
		}
	}
	require.NoError(t, sl.SetExif(rootIb))

	var out bytes.Buffer
	require.NoError(t, sl.Write(&out))
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, out.Bytes(), 0600))
	return path
}

func TestExifService_GetStockInfo(t *testing.T) {
	t.Parallel()

	svc := NewExifService()

	t.Run("plain JPEG has neither tags nor a date", func(t *testing.T) {
		t.Parallel()
		path := writePlainJPEG(t, t.TempDir(), "plain.jpg")

		info, err := svc.GetStockInfo(model.NewPhoto(path))

		require.NoError(t, err)
		assert.Equal(t, StockInfo{}, info)
	})

	t.Run("reads the tags and the shooting date", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithIfdTags(t, t.TempDir(), "tagged.jpg", map[string]map[string]any{
			"IFD0": {
				"ImageDescription": "Lisbon, Portugal - June 13, 2026 A tram climbs the hill.",
				"XPKeywords":       encodeUTF16LE("lisbon;tram;portugal"),
				"DateTime":         "2026:06:14 08:00:00",
			},
			"IFD0/Exif": {
				"DateTimeOriginal": "2026:06:13 17:24:11",
			},
		})

		info, err := svc.GetStockInfo(model.NewPhoto(path))

		require.NoError(t, err)
		assert.Equal(t, "Lisbon, Portugal - June 13, 2026 A tram climbs the hill.", info.Tags.Title)
		assert.Equal(t, []string{"lisbon", "tram", "portugal"}, info.Tags.Keywords)
		assert.Equal(t, time.Date(2026, time.June, 13, 17, 24, 11, 0, time.UTC), info.Taken)
	})

	t.Run("falls back to DateTime when the original is missing", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithIfdTags(t, t.TempDir(), "modified.jpg", map[string]map[string]any{
			"IFD0": {"DateTime": "2026:06:14 08:00:00"},
		})

		info, err := svc.GetStockInfo(model.NewPhoto(path))

		require.NoError(t, err)
		assert.Equal(t, time.Date(2026, time.June, 14, 8, 0, 0, 0, time.UTC), info.Taken)
	})

	t.Run("reads the sidecar of the RAW pair", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := writePlainJPEG(t, dir, "DSC001.jpg")
		require.NoError(t, os.WriteFile(filepath.Join(dir, "DSC001.ARW"), []byte("raw"), 0600))
		require.NoError(t, WriteSidecar(filepath.Join(dir, "DSC001.xmp"), model.Tags{
			Title:    "A tram climbs the hill.",
			Keywords: []string{"lisbon", "tram"},
		}))

		info, err := svc.GetStockInfo(model.NewPhoto(path))

		require.NoError(t, err)
		assert.Equal(t, "A tram climbs the hill.", info.Tags.Title)
		assert.Equal(t, []string{"lisbon", "tram"}, info.Tags.Keywords)
	})

	t.Run("prefers the tags of the JPEG over the sidecar", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := writeJPEGWithIfdTags(t, dir, "DSC002.jpg", map[string]map[string]any{
			"IFD0": {
				"ImageDescription": "From the JPEG.",
				"XPKeywords":       encodeUTF16LE("jpeg"),
			},
		})
		require.NoError(t, os.WriteFile(filepath.Join(dir, "DSC002.ARW"), []byte("raw"), 0600))
		require.NoError(t, WriteSidecar(filepath.Join(dir, "DSC002.xmp"), model.Tags{
			Title:    "From the sidecar.",
			Keywords: []string{"sidecar"},
		}))

		info, err := svc.GetStockInfo(model.NewPhoto(path))

		require.NoError(t, err)
		assert.Equal(t, "From the JPEG.", info.Tags.Title)
		assert.Equal(t, []string{"jpeg"}, info.Tags.Keywords)
		assert.Equal(t, model.Tags{Title: "From the sidecar.", Keywords: []string{"sidecar"}}, info.Sidecar)
	})

	t.Run("reports a missing file", func(t *testing.T) {
		t.Parallel()

		_, err := svc.GetStockInfo(model.NewPhoto(filepath.Join(t.TempDir(), "absent.jpg")))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "parsing JPEG")
	})
}

func TestStockInfoFromTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		flat  []exif.ExifTag
		tags  model.Tags
		taken time.Time
	}{
		{
			name: "no relevant tags",
			flat: []exif.ExifTag{{TagName: "Make", Value: "SONY"}},
		},
		{
			name: "trims the title and drops the padding NUL",
			flat: []exif.ExifTag{{TagName: "ImageDescription", Value: "  A quiet morning. \x00"}},
			tags: model.Tags{Title: "A quiet morning."},
		},
		{
			name: "accepts comma separated keywords",
			flat: []exif.ExifTag{{TagName: "XPKeywords", Value: encodeUTF16LE("lake, forest")}},
			tags: model.Tags{Keywords: []string{"lake", "forest"}},
		},
		{
			name: "prefers the original over the digitized date",
			flat: []exif.ExifTag{
				{TagName: "DateTimeDigitized", Value: "2026:06:14 08:00:00"},
				{TagName: "DateTimeOriginal", Value: "2026:06:13 17:24:11"},
			},
			taken: time.Date(2026, time.June, 13, 17, 24, 11, 0, time.UTC),
		},
		{
			name: "ignores an unparsable date",
			flat: []exif.ExifTag{{TagName: "DateTimeOriginal", Value: "0000:00:00 00:00:00"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			info := stockInfoFromTags(tt.flat)
			assert.Equal(t, tt.tags, info.Tags)
			assert.Equal(t, tt.taken, info.Taken)
		})
	}
}

func TestExifString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "plain", exifString("plain\x00"))
	assert.Equal(t, "keywords", exifString(encodeUTF16LE("keywords")))
	assert.Empty(t, exifString(42))
	assert.Empty(t, exifString([]byte{0x41}), "an odd byte count carries no complete code unit")
}
