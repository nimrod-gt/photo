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
	return writeJPEGSizedWithTags(t, dir, name, 8, 6, tags)
}

func writeJPEGSizedWithTags(t *testing.T, dir, name string, w, h int, tags map[string]any) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, makeTestImage(w, h), nil))

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

		thumbnail, rating, err := svc.GetPhotoInfo(path)

		require.NoError(t, err)
		assert.Nil(t, thumbnail)
		assert.Equal(t, 0, rating)
	})

	// EXIF without a Rating tag has to read as unrated, not as an error: it
	// takes the opposite branch from the plain JPEG above, which never reaches
	// the tag lookups at all
	t.Run("EXIF without a rating", func(t *testing.T) {
		path := writeJPEGWithTags(t, t.TempDir(), "oriented.jpg", map[string]any{
			"Orientation": []uint16{6},
		})

		_, rating, err := svc.GetPhotoInfo(path)

		require.NoError(t, err)
		assert.Equal(t, 0, rating)
	})

	t.Run("missing file", func(t *testing.T) {
		_, _, err := svc.GetPhotoInfo(filepath.Join(t.TempDir(), "missing.jpg"))
		assert.Error(t, err)
	})

	t.Run("invalid JPEG", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "broken.jpg")
		require.NoError(t, os.WriteFile(path, []byte("not a jpeg"), 0600))

		_, _, err := svc.GetPhotoInfo(path)
		assert.Error(t, err)
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

func TestExifService_GetPhotoInfo_XMP(t *testing.T) {
	t.Parallel()

	svc := NewExifService()
	ratedExif := map[string]any{"Make": "SONY", "Rating": []uint16{5}}
	ratedPacket := func(t *testing.T, rating int) []byte {
		t.Helper()
		packet, ok := packetWithRating(sonyPacket(200), rating)
		require.True(t, ok)
		return packet
	}

	t.Run("reads the rating of the packet", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacket(t, t.TempDir(), "rated.jpg", map[string]any{"Make": "SONY"}, ratedPacket(t, 3))

		_, rating, err := svc.GetPhotoInfo(path)

		require.NoError(t, err)
		assert.Equal(t, 3, rating)
	})

	t.Run("reads the attribute Lightroom writes", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacket(t, t.TempDir(), "lightroom.jpg", map[string]any{"Make": "SONY"}, xmpPacket(lightroomRatedDocument, 200))

		_, rating, err := svc.GetPhotoInfo(path)

		require.NoError(t, err)
		assert.Equal(t, 3, rating)
	})

	t.Run("the packet wins over the EXIF even at zero", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacket(t, t.TempDir(), "both.jpg", ratedExif, sonyPacket(200))

		_, rating, err := svc.GetPhotoInfo(path)

		require.NoError(t, err)
		assert.Equal(t, 0, rating)
	})

	t.Run("falls back to the EXIF when the packet has no rating", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacket(t, t.TempDir(), "unrated-packet.jpg", ratedExif, xmpPacket(unratedSonyContent(), 200))

		_, rating, err := svc.GetPhotoInfo(path)

		require.NoError(t, err)
		assert.Equal(t, 5, rating)
	})

	t.Run("keeps the EXIF rating when the packet does not parse", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacket(t, t.TempDir(), "broken.jpg", ratedExif, xmpPacket("<x:xmpmeta><unclosed>", 40))

		_, rating, err := svc.GetPhotoInfo(path)

		require.NoError(t, err)
		assert.Equal(t, 5, rating)
	})

	t.Run("reads the packet of a JPEG without EXIF", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacketOnly(t, t.TempDir(), "packet-only.jpg", ratedPacket(t, 2))

		thumbnail, rating, err := svc.GetPhotoInfo(path)

		require.NoError(t, err)
		assert.Nil(t, thumbnail)
		assert.Equal(t, 2, rating)
	})
}
