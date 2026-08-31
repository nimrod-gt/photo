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
)

func exifRootFromFile(jpegPath string) (*exif.Ifd, error) {
	sl, err := segmentsFromFile(jpegPath)
	if err != nil {
		return nil, err
	}
	return exifRootOf(sl), nil
}

func writePlainJPEG(t *testing.T, dir, name string) string {
	t.Helper()
	return writeSizedPlainJPEG(t, filepath.Join(dir, name), 8, 6)
}

func writeSizedPlainJPEG(t *testing.T, path string, w, h int) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, makeTestImage(w, h), nil))
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0600))
	return path
}

func writeJPEGWithTags(t *testing.T, dir, name string, tags map[string]any) string {
	t.Helper()
	return writeJPEGSizedWithTags(t, dir, name, 8, 6, tags)
}

func writeJPEGSizedWithTags(t *testing.T, dir, name string, w, h int, tags map[string]any) string {
	t.Helper()
	return writeSizedJPEGWithIfdTags(t, dir, name, w, h, map[string]map[string]any{"IFD0": tags})
}

func TestExifService_GetPhotoInfo(t *testing.T) {
	t.Parallel()

	svc := NewExifService()

	t.Run("plain JPEG without EXIF", func(t *testing.T) {
		path := writePlainJPEG(t, t.TempDir(), "plain.jpg")

		info, err := svc.GetPhotoInfo(path)

		require.NoError(t, err)
		assert.Nil(t, info.Thumbnail)
		assert.Equal(t, 0, info.Rating)
		assert.False(t, info.Ratable, "no packet to write a rating into")
	})

	t.Run("the camera's packet takes a rating", func(t *testing.T) {
		path := writeJPEGWithPacket(t, t.TempDir(), "sony.jpg", nil, sonyPacket(2000))

		info, err := svc.GetPhotoInfo(path)

		require.NoError(t, err)
		assert.True(t, info.Ratable)
	})

	t.Run("a read-only packet takes none", func(t *testing.T) {
		packet := bytes.Replace(sonyPacket(2000), []byte("end='w'"), []byte("end='r'"), 1)
		path := writeJPEGWithPacket(t, t.TempDir(), "locked.jpg", nil, packet)

		info, err := svc.GetPhotoInfo(path)

		require.NoError(t, err)
		assert.False(t, info.Ratable)
	})

	// EXIF without a Rating tag has to read as unrated, not as an error: it
	// takes the opposite branch from the plain JPEG above, which never reaches
	// the tag lookups at all
	t.Run("EXIF without a rating", func(t *testing.T) {
		path := writeJPEGWithTags(t, t.TempDir(), "oriented.jpg", map[string]any{
			"Orientation": []uint16{6},
		})

		info, err := svc.GetPhotoInfo(path)

		require.NoError(t, err)
		assert.Equal(t, 0, info.Rating)
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := svc.GetPhotoInfo(filepath.Join(t.TempDir(), "missing.jpg"))
		assert.Error(t, err)
	})

	t.Run("invalid JPEG", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "broken.jpg")
		require.NoError(t, os.WriteFile(path, []byte("not a jpeg"), 0600))

		_, err := svc.GetPhotoInfo(path)
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

		info, err := svc.GetPhotoInfo(path)

		require.NoError(t, err)
		assert.Equal(t, 3, info.Rating)
	})

	t.Run("reads the attribute Lightroom writes", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacket(t, t.TempDir(), "lightroom.jpg", map[string]any{"Make": "SONY"}, xmpPacket(lightroomRatedDocument, 200))

		info, err := svc.GetPhotoInfo(path)

		require.NoError(t, err)
		assert.Equal(t, 3, info.Rating)
	})

	t.Run("the packet wins over the EXIF even at zero", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacket(t, t.TempDir(), "both.jpg", ratedExif, sonyPacket(200))

		info, err := svc.GetPhotoInfo(path)

		require.NoError(t, err)
		assert.Equal(t, 0, info.Rating)
	})

	t.Run("falls back to the EXIF when the packet has no rating", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacket(t, t.TempDir(), "unrated-packet.jpg", ratedExif, xmpPacket(unratedSonyContent(), 200))

		info, err := svc.GetPhotoInfo(path)

		require.NoError(t, err)
		assert.Equal(t, 5, info.Rating)
	})

	t.Run("keeps the EXIF rating when the packet does not parse", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacket(t, t.TempDir(), "broken.jpg", ratedExif, xmpPacket("<x:xmpmeta><unclosed>", 40))

		info, err := svc.GetPhotoInfo(path)

		require.NoError(t, err)
		assert.Equal(t, 5, info.Rating)
	})

	t.Run("reads the packet of a JPEG without EXIF", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacketOnly(t, t.TempDir(), "packet-only.jpg", ratedPacket(t, 2))

		info, err := svc.GetPhotoInfo(path)

		require.NoError(t, err)
		assert.Nil(t, info.Thumbnail)
		assert.Equal(t, 2, info.Rating)
	})
}
