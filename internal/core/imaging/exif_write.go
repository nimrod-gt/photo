package imaging

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"unicode"

	exif "github.com/dsoprea/go-exif/v3"

	"photo/internal/core/model"
)

const (
	// Windows Explorer and the stock uploaders read XPKeywords as a semicolon
	// separated list.
	keywordSeparator = ";"
	// A file we create rather than replace is readable by the tools the user
	// points at it later, Lightroom and Bridge included.
	defaultFilePerm = os.FileMode(0o644)

	tagImageDescription = 0x010e
	tagXPTitle          = 0x9c9b
	tagXPKeywords       = 0x9c9e
	typeASCII           = 2
	typeByte            = 1

	tiffHeaderSize = 8
	entrySize      = 12
	inlineSize     = 4
)

// StockWrite tells the caller what the write cost the file, so the user hears it
// in the same breath as the save.
type StockWrite struct {
	Rewritten bool
	// The EXIF has no field for a place, for the concept, nor for the editorial
	// mark, so the fallback path carries the title and the keywords and leaves
	// those three behind. They survive in the sidecar of the photo, where nothing
	// has to fit in a fixed space.
	PlaceDropped     bool
	ConceptDropped   bool
	EditorialDropped bool
}

// WriteStockTags puts the tags into the XMP packet of the JPEG in place when it
// has room for them, which leaves the size, the layout and the directory entry
// of the file as the camera made them. Without such a packet the EXIF carries
// them and the file is replaced, which is reported so the user can be told the
// camera has to re-index it.
func (s *ExifService) WriteStockTags(jpegPath string, tags model.Tags) (StockWrite, error) {
	s.access.Lock()
	defer s.access.Unlock()

	original, err := os.ReadFile(jpegPath)
	if err != nil {
		return StockWrite{}, fmt.Errorf("reading %s: %w", jpegPath, err)
	}
	start, end, err := xmpPacketSpan(original)
	if err != nil {
		return StockWrite{}, fmt.Errorf("writing the tags of %s: %w", jpegPath, err)
	}
	if packet, ok := packetWithTags(original[start:end], tags); ok {
		rewritten, err := writePacketTags(jpegPath, original, start, end, packet, tags)
		return StockWrite{Rewritten: rewritten}, err
	}
	cleared, err := withoutPacketTags(original, start, end, jpegPath)
	if err != nil {
		return StockWrite{}, err
	}
	updated, err := withStockTags(cleared, tags)
	if err != nil {
		return StockWrite{}, fmt.Errorf("writing the tags of %s: %w", jpegPath, err)
	}
	rewritten, err := replaceIfChanged(jpegPath, original, updated)
	return StockWrite{
		Rewritten:        rewritten,
		PlaceDropped:     !tags.Place.IsEmpty(),
		ConceptDropped:   len(strings.TrimSpace(tags.Concept)) != 0,
		EditorialDropped: tags.Editorial.Marked,
	}, err
}

func replaceIfChanged(jpegPath string, original, updated []byte) (bool, error) {
	if bytes.Equal(updated, original) {
		return false, nil
	}
	return true, replaceFileKeepingModTime(jpegPath, updated)
}

// writePacketTags puts the tags where the packet already holds them. The EXIF an
// earlier version of the app wrote is read behind the packet, so a field the
// user cleared would be filled back in from there and could never be deleted:
// that is the one case where the whole file is rewritten, to clear it as well.
func writePacketTags(jpegPath string, original []byte, start, end int, packet []byte, tags model.Tags) (bool, error) {
	if !exifShadows(original, tags) {
		if bytes.Equal(packet, original[start:end]) {
			return false, nil
		}
		return false, patchPacket(jpegPath, start, packet, original[start:end])
	}

	updated, err := withStockTags(slices.Concat(original[:start], packet, original[end:]), tags)
	if err != nil {
		return false, fmt.Errorf("writing the tags of %s: %w", jpegPath, err)
	}
	return replaceIfChanged(jpegPath, original, updated)
}

// The packet wins on read and the EXIF only fills what it lacks, so the EXIF
// hides nothing except a field the tags being written leave empty. Only the
// EXIF segment is parsed, not the file: the in-place patch this decides about
// is a few kilobytes, and a block the parser rejects is a block no reader shows
// either, so it shadows nothing.
func exifShadows(data []byte, tags model.Tags) bool {
	existing := exifStockTags(data)
	if len(strings.TrimSpace(tags.Title)) == 0 && len(strings.TrimSpace(existing.Title)) != 0 {
		return true
	}
	return len(tags.Keywords) == 0 && len(existing.Keywords) != 0
}

func exifStockTags(data []byte) model.Tags {
	start, end, err := exifSegmentSpan(data)
	if err != nil || start == end {
		return model.Tags{}
	}
	flat, _, err := exif.GetFlatExifData(data[start+segmentHeaderSize+len(exifSegmentPrefix):end], nil)
	if err != nil {
		return model.Tags{}
	}
	return stockInfoFromTags(flat).Tags
}

// The EXIF is read behind the packet, so properties the packet still carries -
// ours from an earlier save, or another tool's - would hide what is written
// into the EXIF. They are cleared where the packet allows it; a packet closed to
// updates that carries tags of its own would keep showing them instead, so the
// write is refused rather than reported as a save no reader would show.
func withoutPacketTags(data []byte, start, end int, source string) ([]byte, error) {
	if start == end {
		return data, nil
	}
	cleared, ok := packetWithTags(data[start:end], model.Tags{})
	if !ok {
		return dataWithoutClearablePacket(data, start, end, source)
	}
	if bytes.Equal(cleared, data[start:end]) {
		return data, nil
	}
	return slices.Concat(data[:start], cleared, data[end:]), nil
}

// A packet the parser rejects is one the app reads nothing from - the dialog
// it opens is seeded from the EXIF - so the EXIF is written behind it and shown
// the same way.
func dataWithoutClearablePacket(data []byte, start, end int, source string) ([]byte, error) {
	parsed, err := parseSidecar(data[start:end])
	if err != nil {
		//nolint:nilerr // a packet no reader parses shadows nothing
		return data, nil
	}
	if !parsed.tags().IsEmpty() {
		return nil, fmt.Errorf("the XMP packet of %s is closed to updates and carries a title or "+
			"keywords of its own, which every reader shows in place of the tags", source)
	}
	return data, nil
}

// The EXIF block is never rebuilt: the tags are appended as a new IFD0 behind
// everything the camera wrote, and only the pointer in the TIFF header moves.
// Every existing tag - the MakerNote and the thumbnail included - keeps its
// bytes and its offset, which no EXIF library manages when it re-encodes.
func withStockTags(original []byte, tags model.Tags) ([]byte, error) {
	start, end, err := exifSegmentSpan(original)
	if err != nil {
		return nil, err
	}
	entries := stockEntries(tags)

	var updatedExif []byte
	if start == end {
		written := filledEntries(entries)
		if len(written) == 0 {
			return original, nil
		}
		updatedExif, err = newExif(written)
	} else {
		updatedExif, err = exifWithTags(original[start+segmentHeaderSize+len(exifSegmentPrefix):end], entries)
	}
	if err != nil {
		return nil, err
	}
	return spliceExifSegment(original, start, end, updatedExif)
}

type exifEntry struct {
	tagID    uint16
	tagType  uint16
	unitSize int
	offset   int
	value    []byte
}

// An empty value means the tag is dropped, so a title the user cleared in the
// dialog is cleared in the file as well - the way the XMP sidecar behaves.
// The XP tags are UTF-16LE whatever byte order the file uses, because their type
// is BYTE and the bytes are stored as they are written here. ImageDescription is
// typed ASCII and holds no other alphabet, so a title that leaves that range is
// carried by XPTitle alone rather than written as bytes no reader can decode.
func stockEntries(tags model.Tags) []exifEntry {
	title := strings.TrimSpace(tags.Title)

	description := exifEntry{tagID: tagImageDescription, tagType: typeASCII, unitSize: 1}
	if len(title) != 0 && isASCII(title) {
		description.value = append([]byte(title), 0)
	}

	xpTitle := exifEntry{tagID: tagXPTitle, tagType: typeByte, unitSize: 1}
	if len(title) != 0 {
		xpTitle.value = encodeUTF16LE(title)
	}

	keywords := exifEntry{tagID: tagXPKeywords, tagType: typeByte, unitSize: 1}
	if len(tags.Keywords) != 0 {
		keywords.value = encodeUTF16LE(strings.Join(tags.Keywords, keywordSeparator))
	}

	return []exifEntry{description, xpTitle, keywords}
}

func isASCII(text string) bool {
	return !strings.ContainsFunc(text, func(r rune) bool { return r > unicode.MaxASCII })
}

func filledEntries(entries []exifEntry) []exifEntry {
	return slices.DeleteFunc(slices.Clone(entries), func(entry exifEntry) bool { return len(entry.value) == 0 })
}

func exifWithTags(tiff []byte, entries []exifEntry) ([]byte, error) {
	order, ifdOffset, err := tiffHeader(tiff)
	if err != nil {
		return nil, err
	}
	existing, next, err := readIfd(tiff, order, ifdOffset)
	if err != nil {
		return nil, err
	}
	trimmed := trimAppendedIfd(tiff, order, ifdOffset, existing, next)
	return appendIfd(trimmed, order, mergeEntries(existing, entries), next)
}

func newExif(entries []exifEntry) ([]byte, error) {
	order := binary.ByteOrder(binary.LittleEndian)
	tiff := make([]byte, tiffHeaderSize)
	copy(tiff, "II")
	order.PutUint16(tiff[2:], 42)
	order.PutUint32(tiff[4:], tiffHeaderSize)
	return appendIfd(tiff, order, entries, 0)
}

func tiffHeader(tiff []byte) (binary.ByteOrder, uint32, error) {
	if len(tiff) < tiffHeaderSize {
		return nil, 0, errors.New("the EXIF block is too short")
	}
	var order binary.ByteOrder
	switch {
	case bytes.HasPrefix(tiff, []byte("II")):
		order = binary.LittleEndian
	case bytes.HasPrefix(tiff, []byte("MM")):
		order = binary.BigEndian
	default:
		return nil, 0, errors.New("the EXIF block has no byte order mark")
	}
	if magic := order.Uint16(tiff[2:]); magic != 42 {
		return nil, 0, fmt.Errorf("the EXIF block carries magic %d, expected 42", magic)
	}
	return order, order.Uint32(tiff[4:]), nil
}

// readIfd returns the entries of one IFD with their values already resolved, so
// the caller can move the IFD without caring where the values used to live.
func readIfd(tiff []byte, order binary.ByteOrder, offset uint32) ([]exifEntry, uint32, error) {
	if int(offset)+2 > len(tiff) {
		return nil, 0, errors.New("the EXIF block points past its end")
	}
	count := int(order.Uint16(tiff[offset:]))
	end := int(offset) + 2 + count*entrySize + 4
	if end > len(tiff) {
		return nil, 0, errors.New("the EXIF block is truncated")
	}

	entries := make([]exifEntry, 0, count)
	for i := range count {
		entry, err := readEntry(tiff, order, int(offset)+2+i*entrySize)
		if err != nil {
			return nil, 0, err
		}
		entries = append(entries, entry)
	}
	return entries, order.Uint32(tiff[end-4:]), nil
}

func readEntry(tiff []byte, order binary.ByteOrder, at int) (exifEntry, error) {
	entry := exifEntry{
		tagID:   order.Uint16(tiff[at:]),
		tagType: order.Uint16(tiff[at+2:]),
	}
	entry.unitSize = tagTypeSize(entry.tagType)
	if entry.unitSize == 0 {
		return exifEntry{}, fmt.Errorf("tag 0x%04x has unknown type %d", entry.tagID, entry.tagType)
	}
	units, err := exifCount(int(order.Uint32(tiff[at+4:])))
	if err != nil {
		return exifEntry{}, fmt.Errorf("tag 0x%04x: %w", entry.tagID, err)
	}
	size := entry.unitSize * int(units)
	if size <= inlineSize {
		entry.value = slices.Clone(tiff[at+8 : at+8+size])
		return entry, nil
	}
	entry.offset = int(order.Uint32(tiff[at+8:]))
	if entry.offset+size > len(tiff) {
		return exifEntry{}, fmt.Errorf("tag 0x%04x points past the EXIF block", entry.tagID)
	}
	entry.value = slices.Clone(tiff[entry.offset : entry.offset+size])
	return entry, nil
}

var tagTypeSizes = map[uint16]int{1: 1, 2: 1, 3: 2, 4: 4, 5: 8, 6: 1, 7: 1, 8: 2, 9: 4, 10: 8, 11: 4, 12: 8, 13: 4}

func tagTypeSize(tagType uint16) int {
	return tagTypeSizes[tagType]
}

func mergeEntries(existing, added []exifEntry) []exifEntry {
	merged := slices.Clone(existing)
	for _, entry := range added {
		index := slices.IndexFunc(merged, func(e exifEntry) bool { return e.tagID == entry.tagID })
		switch {
		case len(entry.value) == 0:
			if index >= 0 {
				merged = slices.Delete(merged, index, index+1)
			}
		case index >= 0:
			merged[index] = entry
		default:
			merged = append(merged, entry)
		}
	}
	slices.SortStableFunc(merged, func(a, b exifEntry) int { return int(a.tagID) - int(b.tagID) })
	return merged
}

// trimAppendedIfd drops the IFD0 an earlier save appended, so that saving the
// tags again reuses its bytes instead of growing the block every time. The tail
// is only cut when it holds nothing but that IFD and the values it owns.
func trimAppendedIfd(tiff []byte, order binary.ByteOrder, ifdOffset uint32, entries []exifEntry, next uint32) []byte {
	if ifdOffset <= tiffHeaderSize || next >= ifdOffset {
		return tiff
	}
	tail := int(ifdOffset) + 2 + len(entries)*entrySize + 4
	for _, entry := range entries {
		if pointsPastIfd(entry, order, ifdOffset) {
			return tiff
		}
		if len(entry.value) <= inlineSize {
			continue
		}
		if entry.offset < int(ifdOffset) {
			return tiff
		}
		tail = max(tail, entry.offset+len(entry.value))
	}
	if tail < len(tiff)-1 {
		return tiff
	}
	return tiff[:ifdOffset]
}

var ifdPointerTags = []uint16{0x8769, 0x8825, 0xa005}

// A sub-IFD is reached through a four byte offset stored inline, so the loop
// over the values the IFD owns never sees it. Cutting the tail would take the
// Exif or GPS block with it and leave the pointer aimed at rewritten bytes.
func pointsPastIfd(entry exifEntry, order binary.ByteOrder, ifdOffset uint32) bool {
	if !slices.Contains(ifdPointerTags, entry.tagID) || len(entry.value) < inlineSize {
		return false
	}
	return order.Uint32(entry.value) >= ifdOffset
}

// appendIfd writes the IFD and the values it owns behind the existing block and
// points the TIFF header at them. What the old IFD left behind stays as unused
// bytes, which readers never reach.
func appendIfd(tiff []byte, order binary.ByteOrder, entries []exifEntry, next uint32) ([]byte, error) {
	count, err := ifdCount(len(entries))
	if err != nil {
		return nil, err
	}

	out := slices.Clone(tiff)
	if len(out)%2 != 0 {
		out = append(out, 0)
	}
	ifdOffset, err := exifOffset(len(out))
	if err != nil {
		return nil, err
	}

	tableSize := 2 + len(entries)*entrySize + 4
	table := make([]byte, tableSize)
	order.PutUint16(table, count)
	order.PutUint32(table[tableSize-4:], next)
	out = append(out, table...)

	for i, entry := range entries {
		out, err = writeEntry(out, order, int(ifdOffset)+2+i*entrySize, entry)
		if err != nil {
			return nil, err
		}
	}

	order.PutUint32(out[4:], ifdOffset)
	return out, nil
}

func writeEntry(out []byte, order binary.ByteOrder, at int, entry exifEntry) ([]byte, error) {
	units, err := exifCount(len(entry.value) / entry.unitSize)
	if err != nil {
		return nil, err
	}
	order.PutUint16(out[at:], entry.tagID)
	order.PutUint16(out[at+2:], entry.tagType)
	order.PutUint32(out[at+4:], units)
	if len(entry.value) <= inlineSize {
		copy(out[at+8:at+8+inlineSize], entry.value)
		return out, nil
	}

	if len(out)%2 != 0 {
		out = append(out, 0)
	}
	valueOffset, err := exifOffset(len(out))
	if err != nil {
		return nil, err
	}
	order.PutUint32(out[at+8:], valueOffset)
	return append(out, entry.value...), nil
}

// Everything an EXIF block addresses has to fit into one JPEG segment, which
// keeps every offset and count far inside their 32 bits.
func exifOffset(value int) (uint32, error) {
	if value < 0 || value > maxSegmentSize {
		return 0, fmt.Errorf("the EXIF needs %d bytes, a JPEG segment holds %d", value, maxSegmentSize)
	}
	return uint32(value), nil
}

func exifCount(value int) (uint32, error) {
	if value < 0 || value > maxSegmentSize {
		return 0, fmt.Errorf("a tag holds %d values, a JPEG segment holds %d bytes", value, maxSegmentSize)
	}
	return uint32(value), nil
}

func ifdCount(value int) (uint16, error) {
	if value < 0 || value > math.MaxUint16 {
		return 0, fmt.Errorf("an IFD holds %d tags, got %d", math.MaxUint16, value)
	}
	return uint16(value), nil
}
