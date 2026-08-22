package imaging

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"photo/internal/core/model"
)

const (
	// The document an ILCE-7M3 writes, byte for byte: single quotes, a byte
	// order mark in the header and no newline behind the last element.
	sonyXMPContent = "<?xpacket begin='\ufeff' id='W5M0MpCehiHzreSzNTczkc9d'?>\n" +
		"<x:xmpmeta xmlns:x='adobe:ns:meta/' x:xmptk=''>\n" +
		"<rdf:RDF xmlns:rdf='http://www.w3.org/1999/02/22-rdf-syntax-ns#'>\n" +
		" <rdf:Description rdf:about=''\n" +
		"  xmlns:xmp='http://ns.adobe.com/xap/1.0/'>\n" +
		"  <xmp:Rating>0</xmp:Rating>\n" +
		" </rdf:Description>\n" +
		"</rdf:RDF>\n" +
		"</x:xmpmeta>"
	xpacketEnd  = "<?xpacket end='w'?>"
	sonyPadding = 56981
)

func xmpPacket(content string, padding int) []byte {
	return slices.Concat([]byte(content), xmpPadding(padding), []byte(xpacketEnd))
}

func sonyPacket(padding int) []byte {
	return xmpPacket(sonyXMPContent, padding)
}

func xmpSegment(packet []byte) []byte {
	header := []byte{markerStart, markerAPP1}
	header = binary.BigEndian.AppendUint16(header, uint16(2+len(xmpSegmentPrefix)+len(packet)))
	return slices.Concat(header, xmpSegmentPrefix, packet)
}

// writeJPEGWithPacket lays the file out the way a camera does: the EXIF first
// and the XMP packet right behind it.
func writeJPEGWithPacket(t *testing.T, dir, name string, exifTags map[string]any, packet []byte) string {
	t.Helper()
	data, err := os.ReadFile(writeJPEGWithTags(t, dir, "source-"+name, exifTags))
	require.NoError(t, err)
	_, end, err := exifSegmentSpan(data)
	require.NoError(t, err)
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, slices.Concat(data[:end], xmpSegment(packet), data[end:]), 0600))
	return path
}

func TestPacketWithTags(t *testing.T) {
	t.Parallel()

	tags := model.Tags{Title: "A tram climbs the hill.", Keywords: []string{"lisbon", "tram"}}

	t.Run("writes into the padding and keeps the length", func(t *testing.T) {
		t.Parallel()
		packet := sonyPacket(2000)

		updated, ok := packetWithTags(packet, tags)

		require.True(t, ok)
		assert.Len(t, updated, len(packet))
		closeAt := bytes.Index(packet, []byte(rdfEnd))
		assert.Equal(t, packet[:closeAt], updated[:closeAt], "everything in front of the appended description must keep its bytes")
		assert.True(t, bytes.HasSuffix(updated, []byte(xpacketEnd)))
		requireWellFormed(t, updated)
		parsed, err := parseSidecar(updated)
		require.NoError(t, err)
		assert.Equal(t, tags, parsed.tags())
	})

	t.Run("writing the same tags again changes nothing", func(t *testing.T) {
		t.Parallel()
		first, ok := packetWithTags(sonyPacket(2000), tags)
		require.True(t, ok)

		second, ok := packetWithTags(first, tags)

		require.True(t, ok)
		assert.Equal(t, first, second)
	})

	t.Run("replaces the tags of an earlier save", func(t *testing.T) {
		t.Parallel()
		first, ok := packetWithTags(sonyPacket(2000), tags)
		require.True(t, ok)
		other := model.Tags{Title: "A quiet morning.", Keywords: []string{"lake", "fog"}}

		second, ok := packetWithTags(first, other)

		require.True(t, ok)
		assert.Len(t, second, len(first))
		text := string(second)
		assert.Equal(t, 2, strings.Count(text, "<rdf:Description"), "the earlier description must not pile up")
		assert.NotContains(t, text, "lisbon")
		assert.Equal(t, bytes.Index(first, []byte(ratingOpen)), bytes.Index(second, []byte(ratingOpen)))
		parsed, err := parseSidecar(second)
		require.NoError(t, err)
		assert.Equal(t, other, parsed.tags())
	})

	t.Run("clearing hands back the original bytes", func(t *testing.T) {
		t.Parallel()
		packet := sonyPacket(2000)
		first, ok := packetWithTags(packet, tags)
		require.True(t, ok)

		cleared, ok := packetWithTags(first, model.Tags{})

		require.True(t, ok)
		assert.Equal(t, packet, cleared)
	})

	t.Run("clearing a packet without tags keeps its own padding", func(t *testing.T) {
		t.Parallel()
		packet := slices.Concat([]byte(sonyXMPContent), bytes.Repeat([]byte{' '}, 300), []byte(xpacketEnd))

		cleared, ok := packetWithTags(packet, model.Tags{})

		require.True(t, ok)
		assert.Equal(t, packet, cleared)
	})

	t.Run("fills the packet to its last byte", func(t *testing.T) {
		t.Parallel()
		generous, ok := packetWithTags(sonyPacket(2000), tags)
		require.True(t, ok)
		trailerAt := bytes.Index(generous, []byte(xpacketEnd))
		require.NotEqual(t, -1, trailerAt)
		content := bytes.TrimRight(generous[:trailerAt], " \n")
		needed := len(content) - len(sonyXMPContent)

		updated, ok := packetWithTags(sonyPacket(needed), tags)

		require.True(t, ok)
		assert.Equal(t, slices.Concat(content, []byte(xpacketEnd)), updated)
		_, ok = packetWithTags(sonyPacket(needed-1), tags)
		assert.False(t, ok, "one byte short must fall back")
	})

	t.Run("accepts a double quoted trailer", func(t *testing.T) {
		t.Parallel()
		packet := slices.Concat([]byte(sonyXMPContent), xmpPadding(2000), []byte(`<?xpacket end="w"?>`))

		updated, ok := packetWithTags(packet, tags)

		require.True(t, ok)
		assert.Len(t, updated, len(packet))
		assert.True(t, bytes.HasSuffix(updated, []byte(`<?xpacket end="w"?>`)))
	})

	t.Run("keeps what sits behind the trailer", func(t *testing.T) {
		t.Parallel()
		packet := slices.Concat(sonyPacket(2000), []byte("\n\x00"))

		updated, ok := packetWithTags(packet, tags)

		require.True(t, ok)
		assert.Len(t, updated, len(packet))
		assert.True(t, bytes.HasSuffix(updated, []byte(xpacketEnd+"\n\x00")))
	})

	t.Run("moves the properties of another tool into a description of its own", func(t *testing.T) {
		t.Parallel()
		packet := []byte(strings.Replace(lightroomSidecar, `<?xpacket end="w"?>`,
			string(xmpPadding(2000))+`<?xpacket end="w"?>`, 1))

		updated, ok := packetWithTags(packet, tags)

		require.True(t, ok)
		assert.Len(t, updated, len(packet))
		text := string(updated)
		assert.Contains(t, text, `crs:Exposure2012="+0.35"`, "the develop settings must survive")
		assert.NotContains(t, text, "Old title")
		assert.NotContains(t, text, ">old<")
		assert.Equal(t, 2, strings.Count(text, "<rdf:Description"))
		requireWellFormed(t, updated)
		parsed, err := parseSidecar(updated)
		require.NoError(t, err)
		assert.Equal(t, tags, parsed.tags())
	})

	refused := []struct {
		name   string
		packet []byte
	}{
		{"a read-only packet", slices.Concat([]byte(sonyXMPContent), xmpPadding(2000), []byte("<?xpacket end='r'?>"))},
		{"a packet without the trailer", slices.Concat([]byte(sonyXMPContent), xmpPadding(2000))},
		{"a packet without rdf:RDF", xmpPacket("<x:xmpmeta xmlns:x='adobe:ns:meta/'/>", 2000)},
		{"a packet without room", sonyPacket(16)},
		{"no packet at all", nil},
	}
	for _, tt := range refused {
		t.Run("leaves alone "+tt.name, func(t *testing.T) {
			t.Parallel()
			_, ok := packetWithTags(tt.packet, tags)
			assert.False(t, ok)
		})
	}
}

func TestXMPPadding(t *testing.T) {
	t.Parallel()

	for _, size := range []int{0, 1, 2, 3, 100, 101, 102, 250, sonyPadding} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			t.Parallel()
			padding := xmpPadding(size)
			require.Len(t, padding, size)
			for at, b := range padding {
				newline := at == size-1 || at%(paddingLineWidth+1) == 0
				assert.Equal(t, newline, b == '\n', "byte %d", at)
				assert.True(t, b == ' ' || b == '\n', "byte %d is %q", at, b)
			}
		})
	}

	t.Run("lays the padding out the way the camera does", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, 566, bytes.Count(xmpPadding(sonyPadding), []byte{'\n'}))
	})
}

func TestXMPPacketSpan(t *testing.T) {
	t.Parallel()

	t.Run("a plain JPEG carries no packet", func(t *testing.T) {
		t.Parallel()
		data, err := os.ReadFile(writePlainJPEG(t, t.TempDir(), "plain.jpg"))
		require.NoError(t, err)

		start, end, err := xmpPacketSpan(data)

		require.NoError(t, err)
		assert.Equal(t, start, end)
	})

	t.Run("finds the packet behind the EXIF", func(t *testing.T) {
		t.Parallel()
		packet := sonyPacket(200)
		data, err := os.ReadFile(writeJPEGWithPacket(t, t.TempDir(), "sony.jpg", map[string]any{"Make": "SONY"}, packet))
		require.NoError(t, err)

		start, end, err := xmpPacketSpan(data)

		require.NoError(t, err)
		assert.Equal(t, packet, data[start:end])
	})

	t.Run("refuses what is not a JPEG", func(t *testing.T) {
		t.Parallel()
		_, _, err := xmpPacketSpan([]byte("not a JPEG at all"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a JPEG")
	})

	t.Run("reports a truncated segment", func(t *testing.T) {
		t.Parallel()
		_, _, err := xmpPacketSpan([]byte{markerStart, markerSOI, markerStart, markerAPP1, 0x10, 0x00, 'x'})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "truncated")
	})
}

func TestPatchFileKeepingModTime(t *testing.T) {
	t.Parallel()

	t.Run("replaces the bytes at the offset and nothing else", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "data.bin")
		require.NoError(t, os.WriteFile(path, []byte("0123456789"), 0600))
		taken := time.Date(2024, time.June, 13, 10, 30, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(path, taken, taken))

		require.NoError(t, patchFileKeepingModTime(path, 3, []byte("abc")))

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "012abc6789", string(data))
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.True(t, info.ModTime().Equal(taken), "got %s", info.ModTime())
	})

	t.Run("reports a missing file", func(t *testing.T) {
		t.Parallel()
		err := patchFileKeepingModTime(filepath.Join(t.TempDir(), "absent.bin"), 0, []byte("x"))
		require.ErrorIs(t, err, os.ErrNotExist)
	})
}

// The packet is written over its own bytes, so a rewrite that came back a
// different length would run into the segment behind it.
func TestPatchPacket(t *testing.T) {
	t.Parallel()

	t.Run("writes a packet of the same length", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "data.bin")
		require.NoError(t, os.WriteFile(path, []byte("0123456789"), 0600))

		require.NoError(t, patchPacket(path, 3, 6, []byte("abc")))

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "012abc6789", string(data))
	})

	t.Run("refuses another length and leaves the file alone", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "data.bin")
		require.NoError(t, os.WriteFile(path, []byte("0123456789"), 0600))

		err := patchPacket(path, 3, 6, []byte("abcd"))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "would change size from 3 to 4 bytes")
		data, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, "0123456789", string(data))
	})
}

// A description Lightroom writes: every property an attribute, the element
// self-closing, the rating in single quotes.
const lightroomRatedDocument = `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about=""
    xmlns:xmp="http://ns.adobe.com/xap/1.0/"
    xmlns:crs="http://ns.adobe.com/camera-raw-settings/1.0/"
    xmp:Rating='3'
    crs:Exposure2012="+0.35"/>
 </rdf:RDF>
</x:xmpmeta>`

func unratedSonyContent() string {
	return strings.Replace(sonyXMPContent, "  <xmp:Rating>0</xmp:Rating>\n", "", 1)
}

func differingBytes(a, b []byte) int {
	count := 0
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			count++
		}
	}
	return count + max(len(a), len(b)) - min(len(a), len(b))
}

func packetRating(t *testing.T, packet []byte) (int, bool) {
	t.Helper()
	parsed, err := parseSidecar(packet)
	require.NoError(t, err)
	return parsed.rating, parsed.rated
}

func TestPacketWithRating(t *testing.T) {
	t.Parallel()

	t.Run("flips the digit the camera wrote and nothing else", func(t *testing.T) {
		t.Parallel()
		packet := sonyPacket(2000)

		updated, ok := packetWithRating(packet, favoriteRating)

		require.True(t, ok)
		require.Len(t, updated, len(packet))
		at := bytes.Index(packet, []byte(ratingOpen)) + len(ratingOpen)
		assert.Equal(t, byte('0'), packet[at])
		assert.Equal(t, byte('5'), updated[at])
		assert.Equal(t, 1, differingBytes(packet, updated))
		rating, rated := packetRating(t, updated)
		assert.True(t, rated)
		assert.Equal(t, 5, rating)
	})

	t.Run("keeps foreign padding when the length does not change", func(t *testing.T) {
		t.Parallel()
		packet := slices.Concat([]byte(sonyXMPContent), bytes.Repeat([]byte{' '}, 300), []byte(xpacketEnd))

		updated, ok := packetWithRating(packet, 3)

		require.True(t, ok)
		assert.Equal(t, 1, differingBytes(packet, updated))
	})

	t.Run("clearing hands back the camera's bytes", func(t *testing.T) {
		t.Parallel()
		packet := sonyPacket(2000)
		rated, ok := packetWithRating(packet, favoriteRating)
		require.True(t, ok)

		cleared, ok := packetWithRating(rated, 0)

		require.True(t, ok)
		assert.Equal(t, packet, cleared)
	})

	t.Run("adds the rating to a packet without one", func(t *testing.T) {
		t.Parallel()
		packet := xmpPacket(unratedSonyContent(), 2000)
		_, rated := packetRating(t, packet)
		require.False(t, rated)

		updated, ok := packetWithRating(packet, favoriteRating)

		require.True(t, ok)
		assert.Len(t, updated, len(packet))
		requireWellFormed(t, updated)
		text := string(updated)
		assert.Contains(t, text, ratingDescriptionOpen)
		assert.Equal(t, 2, strings.Count(text, "<rdf:Description"))
		rating, rated := packetRating(t, updated)
		assert.True(t, rated)
		assert.Equal(t, 5, rating)
	})

	t.Run("writes zero out rather than leaving it absent", func(t *testing.T) {
		t.Parallel()
		packet := xmpPacket(unratedSonyContent(), 2000)

		updated, ok := packetWithRating(packet, 0)

		require.True(t, ok)
		assert.Contains(t, string(updated), ratingOpen+"0"+ratingClose)
		rating, rated := packetRating(t, updated)
		assert.True(t, rated)
		assert.Equal(t, 0, rating)
	})

	t.Run("expands the self-closing form", func(t *testing.T) {
		t.Parallel()
		content := strings.Replace(sonyXMPContent, "<xmp:Rating>0</xmp:Rating>", "<xmp:Rating/>", 1)
		packet := xmpPacket(content, 2000)

		updated, ok := packetWithRating(packet, 4)

		require.True(t, ok)
		assert.Len(t, updated, len(packet))
		assert.Equal(t, 1, strings.Count(string(updated), "<xmp:Rating"))
		rating, rated := packetRating(t, updated)
		assert.True(t, rated)
		assert.Equal(t, 4, rating)
	})

	t.Run("rewrites the attribute Lightroom writes and keeps its quotes", func(t *testing.T) {
		t.Parallel()
		packet := xmpPacket(lightroomRatedDocument, 2000)

		updated, ok := packetWithRating(packet, favoriteRating)

		require.True(t, ok)
		assert.Len(t, updated, len(packet))
		text := string(updated)
		assert.Contains(t, text, "xmp:Rating='5'")
		assert.Contains(t, text, `crs:Exposure2012="+0.35"`)
		assert.Equal(t, 1, differingBytes(packet, updated))
		rating, rated := packetRating(t, updated)
		assert.True(t, rated)
		assert.Equal(t, 5, rating)
	})

	t.Run("refuses a rating under a prefix it does not rewrite", func(t *testing.T) {
		t.Parallel()
		content := strings.NewReplacer("xmlns:xmp=", "xmlns:xap=", "<xmp:Rating>", "<xap:Rating>", "</xmp:Rating>", "</xap:Rating>").
			Replace(sonyXMPContent)
		packet := xmpPacket(content, 2000)

		updated, ok := packetWithRating(packet, favoriteRating)

		assert.False(t, ok)
		assert.Nil(t, updated)
	})

	t.Run("a neighbouring property is not taken for a rating", func(t *testing.T) {
		t.Parallel()
		content := strings.Replace(sonyXMPContent,
			"<xmp:Rating>0</xmp:Rating>", "<xmp:RatingPercent>50</xmp:RatingPercent>", 1)
		packet := xmpPacket(content, 2000)

		updated, ok := packetWithRating(packet, favoriteRating)

		require.True(t, ok)
		require.Len(t, updated, len(packet))
		requireWellFormed(t, updated)
		assert.Contains(t, string(updated), "<xmp:RatingPercent>50</xmp:RatingPercent>")
		rating, rated := packetRating(t, updated)
		assert.True(t, rated)
		assert.Equal(t, 5, rating)
	})

	t.Run("a shorter rating grows the padding", func(t *testing.T) {
		t.Parallel()
		content := strings.Replace(sonyXMPContent, "<xmp:Rating>0</xmp:Rating>", "<xmp:Rating>-1</xmp:Rating>", 1)
		packet := xmpPacket(content, 2000)

		updated, ok := packetWithRating(packet, favoriteRating)

		require.True(t, ok)
		require.Len(t, updated, len(packet))
		document := strings.Replace(content, "-1", "5", 1)
		assert.Equal(t, xmpPacket(document, 2001), updated)
	})

	t.Run("survives the tags being written and cleared", func(t *testing.T) {
		t.Parallel()
		packet := xmpPacket(unratedSonyContent(), 2000)
		rated, ok := packetWithRating(packet, favoriteRating)
		require.True(t, ok)
		tags := model.Tags{Title: "A tram climbs the hill.", Keywords: []string{"lisbon"}}
		tagged, ok := packetWithTags(rated, tags)
		require.True(t, ok)
		parsed, err := parseSidecar(tagged)
		require.NoError(t, err)
		assert.Equal(t, tags, parsed.tags())
		assert.Equal(t, 5, parsed.rating)

		cleared, ok := packetWithTags(tagged, model.Tags{})

		require.True(t, ok)
		assert.Equal(t, rated, cleared)
	})

	t.Run("keeps the tags written before it", func(t *testing.T) {
		t.Parallel()
		tags := model.Tags{Title: "A quiet morning.", Keywords: []string{"lake", "fog"}}
		tagged, ok := packetWithTags(sonyPacket(2000), tags)
		require.True(t, ok)

		rated, ok := packetWithRating(tagged, favoriteRating)

		require.True(t, ok)
		assert.Len(t, rated, len(tagged))
		assert.Equal(t, 1, differingBytes(tagged, rated))
		parsed, err := parseSidecar(rated)
		require.NoError(t, err)
		assert.Equal(t, tags, parsed.tags())
		assert.Equal(t, 5, parsed.rating)
	})

	refused := []struct {
		name   string
		packet []byte
	}{
		{"a read-only packet", slices.Concat([]byte(sonyXMPContent), xmpPadding(2000), []byte("<?xpacket end='r'?>"))},
		{"a packet without the trailer", slices.Concat([]byte(sonyXMPContent), xmpPadding(2000))},
		{"a packet without rdf:RDF", xmpPacket("<x:xmpmeta xmlns:x='adobe:ns:meta/'/>", 2000)},
		{"a packet without room", xmpPacket(unratedSonyContent(), 16)},
		{"no packet at all", nil},
	}
	for _, tt := range refused {
		t.Run("leaves alone "+tt.name, func(t *testing.T) {
			t.Parallel()
			_, ok := packetWithRating(tt.packet, favoriteRating)
			assert.False(t, ok)
		})
	}
}
