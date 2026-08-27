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

	"photo/internal/core/filedate"
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

	t.Run("plain JPEG has no tags and is dated by its file", func(t *testing.T) {
		t.Parallel()
		path := writePlainJPEG(t, t.TempDir(), "plain.jpg")

		info, err := svc.GetStockInfo(model.NewPhoto(path))

		require.NoError(t, err)
		assert.Equal(t, model.Tags{}, info.Tags)
		assert.Equal(t, filedate.Created(path), info.Taken)
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

	t.Run("prefers the tags of the sidecar over the JPEG", func(t *testing.T) {
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
		assert.Equal(t, "From the sidecar.", info.Tags.Title)
		assert.Equal(t, []string{"sidecar"}, info.Tags.Keywords)
	})

	t.Run("falls back to the JPEG for what the sidecar lacks", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := writeJPEGWithIfdTags(t, dir, "DSC003.jpg", map[string]map[string]any{
			"IFD0": {
				"ImageDescription": "From the JPEG.",
				"XPKeywords":       encodeUTF16LE("jpeg"),
			},
		})
		require.NoError(t, os.WriteFile(filepath.Join(dir, "DSC003.ARW"), []byte("raw"), 0600))
		require.NoError(t, WriteSidecar(filepath.Join(dir, "DSC003.xmp"), model.Tags{
			Keywords: []string{"sidecar"},
		}))

		info, err := svc.GetStockInfo(model.NewPhoto(path))

		require.NoError(t, err)
		assert.Equal(t, "From the JPEG.", info.Tags.Title)
		assert.Equal(t, []string{"sidecar"}, info.Tags.Keywords)
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

func TestExifService_GetStockInfo_XMP(t *testing.T) {
	t.Parallel()

	svc := NewExifService()
	fromPacket := model.Tags{Title: "From the packet.", Keywords: []string{"packet"}}
	exifTags := map[string]any{
		"ImageDescription": "From the EXIF.",
		"XPKeywords":       encodeUTF16LE("exif"),
		"DateTime":         "2026:06:14 08:00:00",
	}
	packetWith := func(t *testing.T, tags model.Tags) []byte {
		t.Helper()
		packet, ok := packetWithTags(sonyPacket(2000), tags)
		require.True(t, ok)
		return packet
	}

	t.Run("prefers the packet over the EXIF", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacket(t, t.TempDir(), "both.jpg", exifTags, packetWith(t, fromPacket))

		info, err := svc.GetStockInfo(model.NewPhoto(path))

		require.NoError(t, err)
		assert.Equal(t, fromPacket, info.Tags)
		assert.Equal(t, time.Date(2026, time.June, 14, 8, 0, 0, 0, time.UTC), info.Taken, "the date still comes from the EXIF")
	})

	t.Run("fills from the EXIF what the packet lacks", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacket(t, t.TempDir(), "partial.jpg", exifTags, packetWith(t, model.Tags{Keywords: []string{"packet"}}))

		info, err := svc.GetStockInfo(model.NewPhoto(path))

		require.NoError(t, err)
		assert.Equal(t, model.Tags{Title: "From the EXIF.", Keywords: []string{"packet"}}, info.Tags)
	})

	t.Run("prefers the sidecar over the packet", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := writeJPEGWithPacket(t, dir, "DSC010.jpg", exifTags, packetWith(t, fromPacket))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "DSC010.ARW"), []byte("raw"), 0600))
		require.NoError(t, WriteSidecar(filepath.Join(dir, "DSC010.xmp"), model.Tags{Title: "From the sidecar."}))

		info, err := svc.GetStockInfo(model.NewPhoto(path))

		require.NoError(t, err)
		assert.Equal(t, model.Tags{Title: "From the sidecar.", Keywords: []string{"packet"}}, info.Tags)
	})

	t.Run("prefers the sidecar of a photo with no RAW pair over the packet", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := writeJPEGWithPacket(t, dir, "DSC011.jpg", exifTags, packetWith(t, fromPacket))
		require.NoError(t, WriteSidecar(filepath.Join(dir, "DSC011.xmp"), model.Tags{Title: "From the sidecar."}))

		photo := model.NewPhoto(path)
		require.False(t, photo.HasRAW())
		info, err := svc.GetStockInfo(photo)

		require.NoError(t, err)
		assert.Equal(t, model.Tags{Title: "From the sidecar.", Keywords: []string{"packet"}}, info.Tags)
	})

	// A PNG carries no packet and no EXIF of ours, so its sidecar is all it has.
	t.Run("reads the sidecar of a photo that is no JPEG", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "shot.png")
		require.NoError(t, os.WriteFile(path, []byte("not a real png"), 0600))
		saved := model.Tags{Title: "From the sidecar.", Keywords: []string{"sidecar"}}
		require.NoError(t, WriteSidecar(filepath.Join(dir, "shot.xmp"), saved))

		info, err := svc.GetStockInfo(model.NewPhoto(path))

		require.NoError(t, err)
		assert.Equal(t, saved, info.Tags)
	})

	// The EXIF has no field for a place, so a JPEG whose packet had no room keeps
	// its location in the sidecar alone - and a sidecar another tool wrote may
	// have tags and no location while the packet has one.
	t.Run("fills from the packet the place the sidecar lacks", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		placed := model.Tags{Title: "From the packet.", Place: model.Place{Location: "Praia do Guincho", City: "Cascais"}}
		path := writeJPEGWithPacket(t, dir, "DSC020.jpg", exifTags, packetWith(t, placed))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "DSC020.ARW"), []byte("raw"), 0600))
		require.NoError(t, WriteSidecar(filepath.Join(dir, "DSC020.xmp"), model.Tags{Title: "From the sidecar."}))

		info, err := svc.GetStockInfo(model.NewPhoto(path))

		require.NoError(t, err)
		assert.Equal(t, "From the sidecar.", info.Tags.Title)
		assert.Equal(t, model.Place{Location: "Praia do Guincho", City: "Cascais"}, info.Tags.Place)
	})

	// The concept is written wherever the tags are, so a JPEG carries it in its
	// packet and a photo with a RAW pair in its sidecar; the packet fills what a
	// sidecar another tool wrote has nothing to say about.
	t.Run("fills from the packet the concept the sidecar lacks", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		conceived := model.Tags{Title: "From the packet.", Concept: "tram 28 seen head-on"}
		path := writeJPEGWithPacket(t, dir, "DSC030.jpg", exifTags, packetWith(t, conceived))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "DSC030.ARW"), []byte("raw"), 0600))
		require.NoError(t, WriteSidecar(filepath.Join(dir, "DSC030.xmp"), model.Tags{Title: "From the sidecar."}))

		info, err := svc.GetStockInfo(model.NewPhoto(path))

		require.NoError(t, err)
		assert.Equal(t, "From the sidecar.", info.Tags.Title)
		assert.Equal(t, "tram 28 seen head-on", info.Tags.Concept)
	})

	t.Run("prefers the concept of the sidecar over the one in the packet", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		conceived := model.Tags{Title: "From the packet.", Concept: "the packet's idea"}
		path := writeJPEGWithPacket(t, dir, "DSC031.jpg", exifTags, packetWith(t, conceived))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "DSC031.ARW"), []byte("raw"), 0600))
		require.NoError(t, WriteSidecar(filepath.Join(dir, "DSC031.xmp"),
			model.Tags{Title: "From the sidecar.", Concept: "the sidecar's idea"}))

		info, err := svc.GetStockInfo(model.NewPhoto(path))

		require.NoError(t, err)
		assert.Equal(t, "the sidecar's idea", info.Tags.Concept)
	})

	t.Run("reports a packet it cannot parse and keeps the EXIF tags", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacket(t, t.TempDir(), "broken.jpg", exifTags, xmpPacket("<x:xmpmeta><unclosed>", 40))

		info, err := svc.GetStockInfo(model.NewPhoto(path))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "XMP")
		assert.Equal(t, "From the EXIF.", info.Tags.Title)
	})
}

// The sidecar is the store the dialog writes on its own. A JPEG packet the
// parser rejects used to end the read before it, and the dialog was seeded from
// the EXIF instead - the first save then wrote that over the tags in the sidecar.
func TestExifService_GetStockInfo_ReadsTheSidecarBehindABrokenPacket(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeJPEGWithPacket(t, dir, "DSC010.jpg", map[string]any{
		"Make":             "SONY",
		"ImageDescription": "The title in the EXIF.",
	}, xmpPacket("<x:xmpmeta><unclosed>", 2000))
	rawPath := filepath.Join(dir, "DSC010.ARW")
	require.NoError(t, os.WriteFile(rawPath, []byte("not a real raw"), 0600))
	saved := model.Tags{Title: "Saved earlier.", Keywords: []string{"lisbon", "tram"}}
	require.NoError(t, WriteSidecar(model.SidecarPath(rawPath), saved))

	photo := model.NewPhoto(path)
	require.True(t, photo.HasRAW())
	info, err := NewExifService().GetStockInfo(photo)

	require.Error(t, err, "the unreadable packet is still reported")
	assert.Equal(t, saved, info.Tags)
}
