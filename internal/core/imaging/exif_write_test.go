package imaging

import (
	"bytes"
	"encoding/binary"
	"image/jpeg"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

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

	t.Run("keeps the modification time of the original", func(t *testing.T) {
		t.Parallel()
		path := writePlainJPEG(t, t.TempDir(), "modtime.jpg")
		taken := time.Date(2024, time.June, 13, 10, 30, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(path, taken, taken))

		require.NoError(t, svc.WriteStockTags(path, written))

		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.True(t, info.ModTime().Equal(taken),
			"writing tags must not move the photo under the time sort, got %s", info.ModTime())
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

	t.Run("carries a title outside ASCII in XPTitle alone", func(t *testing.T) {
		t.Parallel()
		path := writePlainJPEG(t, t.TempDir(), "accented.jpg")
		accented := model.Tags{Title: "Un cafe\u0301 sur la place, Montre\u0301al.", Keywords: []string{"cafe"}}

		require.NoError(t, svc.WriteStockTags(path, accented))

		info, err := svc.GetStockInfo(model.NewPhoto(path))
		require.NoError(t, err)
		assert.Equal(t, accented, info.Tags)

		names := tagNamesOf(t, path)
		assert.Contains(t, names, "XPTitle")
		assert.NotContains(t, names, "ImageDescription",
			"a tag typed ASCII must not be handed bytes outside ASCII")
	})

	t.Run("writes a title inside ASCII into both title tags", func(t *testing.T) {
		t.Parallel()
		path := writePlainJPEG(t, t.TempDir(), "ascii.jpg")

		require.NoError(t, svc.WriteStockTags(path, written))

		names := tagNamesOf(t, path)
		assert.Contains(t, names, "ImageDescription")
		assert.Contains(t, names, "XPTitle")
	})

	t.Run("clears both title tags when the title is emptied", func(t *testing.T) {
		t.Parallel()
		path := writePlainJPEG(t, t.TempDir(), "cleared.jpg")
		require.NoError(t, svc.WriteStockTags(path, written))

		require.NoError(t, svc.WriteStockTags(path, model.Tags{Keywords: written.Keywords}))

		names := tagNamesOf(t, path)
		assert.NotContains(t, names, "ImageDescription")
		assert.NotContains(t, names, "XPTitle")
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

func tagNamesOf(t *testing.T, path string) []string {
	t.Helper()
	flat, err := flatExifFromFile(path)
	require.NoError(t, err)
	names := make([]string, 0, len(flat))
	for _, tag := range flat {
		names = append(names, tag.TagName)
	}
	return names
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

func TestExifService_WriteStockTags_Clears(t *testing.T) {
	t.Parallel()

	svc := NewExifService()
	path := writeJPEGWithTags(t, t.TempDir(), "camera.jpg", map[string]any{"Make": "SONY"})
	require.NoError(t, svc.WriteStockTags(path, model.Tags{
		Title:    "The title written first.",
		Keywords: []string{"lake", "fog"},
	}))

	t.Run("drops the title the user emptied", func(t *testing.T) {
		require.NoError(t, svc.WriteStockTags(path, model.Tags{Keywords: []string{"lake"}}))

		info, err := svc.GetStockInfo(model.NewPhoto(path))
		require.NoError(t, err)
		assert.Empty(t, info.Tags.Title)
		assert.Equal(t, []string{"lake"}, info.Tags.Keywords)
	})

	t.Run("drops the keywords the user emptied", func(t *testing.T) {
		require.NoError(t, svc.WriteStockTags(path, model.Tags{Title: "A second title."}))

		info, err := svc.GetStockInfo(model.NewPhoto(path))
		require.NoError(t, err)
		assert.Equal(t, "A second title.", info.Tags.Title)
		assert.Empty(t, info.Tags.Keywords)
	})

	t.Run("keeps what the camera wrote", func(t *testing.T) {
		flat, err := flatExifFromFile(path)
		require.NoError(t, err)
		assert.Contains(t, flatValues(flat), "SONY")
	})
}

func TestExifService_WriteStockTags_LeavesTheFileAloneWithNothingToWrite(t *testing.T) {
	t.Parallel()

	svc := NewExifService()
	path := writePlainJPEG(t, t.TempDir(), "plain.jpg")
	before, err := os.ReadFile(path)
	require.NoError(t, err)

	require.NoError(t, svc.WriteStockTags(path, model.Tags{}))

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, before, after)
}

// JFIF claims the first segment of the file, so the EXIF one goes behind it.
func TestExifService_WriteStockTags_KeepsTheJFIFHeaderFirst(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	plain, err := os.ReadFile(writePlainJPEG(t, dir, "plain.jpg"))
	require.NoError(t, err)
	jfif := []byte{markerStart, markerAPP0, 0, 16, 'J', 'F', 'I', 'F', 0, 1, 2, 0, 0, 1, 0, 1, 0, 0}

	path := filepath.Join(dir, "jfif.jpg")
	withJFIF := slices.Concat(plain[:2], jfif, plain[2:])
	require.NoError(t, os.WriteFile(path, withJFIF, 0600))

	require.NoError(t, NewExifService().WriteStockTags(path, model.Tags{Title: "A title."}))

	updated, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, jfif, updated[2:2+len(jfif)], "the JFIF header must stay the first segment")
	start, _, err := exifSegmentSpan(updated)
	require.NoError(t, err)
	assert.Equal(t, 2+len(jfif), start)
	_, err = jpeg.Decode(bytes.NewReader(updated))
	require.NoError(t, err)
}

const (
	subIfdTableOffset = 16
	subIfdOffset      = 46
)

// tiffWithSubIfd lays out a block whose IFD0 sits past the header with its own
// values ending at the block end - the shape trimAppendedIfd cuts - and reaches
// a sub-IFD through an inline pointer.
func tiffWithSubIfd(pointer uint32) []byte {
	order := binary.LittleEndian
	tiff := make([]byte, 66)
	copy(tiff, "II")
	order.PutUint16(tiff[2:], 42)
	order.PutUint32(tiff[4:], subIfdTableOffset)

	order.PutUint16(tiff[subIfdTableOffset:], 2)
	writeTestEntry(tiff, 18, 0x8769, 4, 1, pointer)
	writeTestEntry(tiff, 30, tagImageDescription, typeASCII, 10, 56)

	copy(tiff[subIfdOffset:], "SUBIFDDATA")
	copy(tiff[56:], "old title\x00")
	return tiff
}

func writeTestEntry(tiff []byte, at int, tagID, tagType uint16, units, value uint32) {
	order := binary.LittleEndian
	order.PutUint16(tiff[at:], tagID)
	order.PutUint16(tiff[at+2:], tagType)
	order.PutUint32(tiff[at+4:], units)
	order.PutUint32(tiff[at+8:], value)
}

func TestTrimAppendedIfd(t *testing.T) {
	t.Parallel()

	order := binary.ByteOrder(binary.LittleEndian)

	t.Run("keeps a block whose sub-IFD sits behind the table", func(t *testing.T) {
		t.Parallel()
		tiff := tiffWithSubIfd(subIfdOffset)
		entries, next, err := readIfd(tiff, order, subIfdTableOffset)
		require.NoError(t, err)

		assert.Equal(t, tiff, trimAppendedIfd(tiff, order, subIfdTableOffset, entries, next))
	})

	t.Run("cuts a block whose sub-IFD sits before the table", func(t *testing.T) {
		t.Parallel()
		tiff := tiffWithSubIfd(8)
		entries, next, err := readIfd(tiff, order, subIfdTableOffset)
		require.NoError(t, err)

		assert.Equal(t, tiff[:subIfdTableOffset], trimAppendedIfd(tiff, order, subIfdTableOffset, entries, next))
	})
}

func TestExifWithTags_KeepsASubIfdBehindTheTable(t *testing.T) {
	t.Parallel()

	tiff := tiffWithSubIfd(subIfdOffset)
	updated, err := exifWithTags(tiff, stockEntries(model.Tags{Title: "New title.", Keywords: []string{"lisbon"}}))
	require.NoError(t, err)

	assert.Equal(t, "SUBIFDDATA", string(updated[subIfdOffset:subIfdOffset+len("SUBIFDDATA")]))

	entries, _, err := readIfd(updated, binary.LittleEndian, binary.LittleEndian.Uint32(updated[4:]))
	require.NoError(t, err)
	pointer, ok := entryByTag(entries, 0x8769)
	require.True(t, ok)
	assert.Equal(t, uint32(subIfdOffset), binary.LittleEndian.Uint32(pointer.value))
}

func entryByTag(entries []exifEntry, tagID uint16) (exifEntry, bool) {
	for _, entry := range entries {
		if entry.tagID == tagID {
			return entry, true
		}
	}
	return exifEntry{}, false
}
