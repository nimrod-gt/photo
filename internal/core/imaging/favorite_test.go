package imaging

import (
	"bytes"
	"image/jpeg"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeJPEGWithPacketOnly lays out a JPEG that carries an XMP packet but no
// EXIF, the way an export from a web tool might.
func writeJPEGWithPacketOnly(t *testing.T, dir, name string, packet []byte) string {
	t.Helper()
	data, err := os.ReadFile(writePlainJPEG(t, dir, "source-"+name))
	require.NoError(t, err)
	insertAt, end, err := exifSegmentSpan(data)
	require.NoError(t, err)
	require.Equal(t, insertAt, end)
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, slices.Concat(data[:insertAt], xmpSegment(packet), data[insertAt:]), 0600))
	return path
}

func mustRating(t *testing.T, svc *ExifService, path string) int {
	t.Helper()
	_, rating, err := svc.GetPhotoInfo(path)
	require.NoError(t, err)
	return rating
}

func mustToggleFavorite(t *testing.T, svc *ExifService, path string) bool {
	t.Helper()
	favorite, err := svc.ToggleFavorite(path)
	require.NoError(t, err)
	return favorite
}

func TestExifService_ToggleFavorite(t *testing.T) {
	t.Parallel()

	svc := NewExifService()
	cameraExif := map[string]any{"Make": "SONY", "Orientation": []uint16{6}}
	ratedExif := map[string]any{"Make": "SONY", "Rating": []uint16{5}}

	t.Run("rates the photo in place and back", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacket(t, t.TempDir(), "sony.jpg", cameraExif, sonyPacket(2000))
		original := readBytes(t, path)
		taken := time.Date(2024, time.June, 13, 10, 30, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(path, taken, taken))
		before, err := os.Stat(path)
		require.NoError(t, err)

		assert.True(t, mustToggleFavorite(t, svc, path))

		rated := readBytes(t, path)
		require.Len(t, rated, len(original))
		assert.Equal(t, 1, differingBytes(original, rated))
		at := bytes.Index(original, []byte(ratingOpen)) + len(ratingOpen)
		assert.Equal(t, byte('5'), rated[at])
		after, err := os.Stat(path)
		require.NoError(t, err)
		assert.True(t, os.SameFile(before, after), "the file must be patched, not replaced")
		assert.True(t, after.ModTime().Equal(taken), "got %s", after.ModTime())
		assert.Equal(t, favoriteRating, mustRating(t, svc, path))
		_, err = jpeg.Decode(bytes.NewReader(rated))
		require.NoError(t, err)

		assert.False(t, mustToggleFavorite(t, svc, path))

		assert.Equal(t, original, readBytes(t, path))
		assert.Equal(t, 0, mustRating(t, svc, path))
	})

	t.Run("the packet wins over an EXIF rating", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacket(t, t.TempDir(), "both.jpg", ratedExif, sonyPacket(2000))
		require.Equal(t, 0, mustRating(t, svc, path))

		assert.True(t, mustToggleFavorite(t, svc, path))

		assert.Equal(t, favoriteRating, mustRating(t, svc, path))
	})

	t.Run("writes an explicit zero over an EXIF rating", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacket(t, t.TempDir(), "exif-rated.jpg", ratedExif, xmpPacket(unratedSonyContent(), 2000))
		require.Equal(t, 5, mustRating(t, svc, path), "the EXIF rating shows while the packet says nothing")
		size := sizeOf(t, path)

		assert.False(t, mustToggleFavorite(t, svc, path))

		assert.Equal(t, 0, mustRating(t, svc, path))
		assert.Equal(t, size, sizeOf(t, path))
		assert.Contains(t, string(packetOf(t, readBytes(t, path))), ratingOpen+"0"+ratingClose)
		assert.True(t, mustToggleFavorite(t, svc, path))
		assert.Equal(t, favoriteRating, mustRating(t, svc, path))
	})

	t.Run("rates a JPEG without EXIF", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacketOnly(t, t.TempDir(), "packet-only.jpg", sonyPacket(2000))

		assert.True(t, mustToggleFavorite(t, svc, path))

		assert.Equal(t, favoriteRating, mustRating(t, svc, path))
	})

	t.Run("serializes toggles of the same file", func(t *testing.T) {
		t.Parallel()
		path := writeJPEGWithPacket(t, t.TempDir(), "busy.jpg", cameraExif, sonyPacket(2000))
		original := readBytes(t, path)
		const toggles = 8

		var wg sync.WaitGroup
		for range toggles {
			wg.Go(func() {
				_, err := svc.ToggleFavorite(path)
				assert.NoError(t, err)
			})
		}
		wg.Wait()

		assert.Equal(t, original, readBytes(t, path), "an even number of toggles must end where it started")
	})

	untouched := []struct {
		name    string
		write   func(t *testing.T, dir string) string
		message string
	}{
		{
			name: "a JPEG without a packet",
			write: func(t *testing.T, dir string) string {
				return writeJPEGWithTags(t, dir, "plain.jpg", cameraExif)
			},
			message: "no XMP packet",
		},
		{
			name: "a read-only packet",
			write: func(t *testing.T, dir string) string {
				packet := slices.Concat([]byte(sonyXMPContent), xmpPadding(2000), []byte("<?xpacket end='r'?>"))
				return writeJPEGWithPacket(t, dir, "readonly.jpg", cameraExif, packet)
			},
			message: "read-only",
		},
		{
			name: "a packet without room",
			write: func(t *testing.T, dir string) string {
				return writeJPEGWithPacket(t, dir, "full.jpg", cameraExif, xmpPacket(unratedSonyContent(), 16))
			},
			message: "no room",
		},
		{
			name: "a rating under a prefix this app does not rewrite",
			write: func(t *testing.T, dir string) string {
				content := strings.NewReplacer("xmlns:xmp=", "xmlns:xap=", "<xmp:Rating>", "<xap:Rating>", "</xmp:Rating>", "</xap:Rating>").
					Replace(sonyXMPContent)
				return writeJPEGWithPacket(t, dir, "xap.jpg", cameraExif, xmpPacket(content, 2000))
			},
			message: "cannot take a rating",
		},
		{
			name: "a packet that does not parse",
			write: func(t *testing.T, dir string) string {
				return writeJPEGWithPacket(t, dir, "broken.jpg", cameraExif, xmpPacket("<x:xmpmeta><unclosed>", 2000))
			},
			message: "reading the rating",
		},
	}
	for _, tt := range untouched {
		t.Run("leaves alone "+tt.name, func(t *testing.T) {
			t.Parallel()
			path := tt.write(t, t.TempDir())
			original := readBytes(t, path)

			_, err := svc.ToggleFavorite(path)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.message)
			assert.Equal(t, original, readBytes(t, path))
		})
	}

	t.Run("reports a missing file", func(t *testing.T) {
		t.Parallel()
		_, err := svc.ToggleFavorite(filepath.Join(t.TempDir(), "absent.jpg"))
		require.ErrorIs(t, err, os.ErrNotExist)
	})
}
