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

	"photo/internal/core/model"
)

const (
	// Windows Explorer and the stock uploaders read XPKeywords as a semicolon
	// separated list.
	keywordSeparator = ";"
	defaultFilePerm  = os.FileMode(0600)

	tagImageDescription = 0x010e
	tagXPKeywords       = 0x9c9e
	typeASCII           = 2
	typeByte            = 1

	tiffHeaderSize = 8
	entrySize      = 12
	inlineSize     = 4
)

func (s *ExifService) WriteStockTags(jpegPath string, tags model.Tags) error {
	original, err := os.ReadFile(jpegPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", jpegPath, err)
	}
	updated, err := withStockTags(original, tags)
	if err != nil {
		return fmt.Errorf("writing the tags of %s: %w", jpegPath, err)
	}
	if bytes.Equal(updated, original) {
		return nil
	}
	return replaceFile(jpegPath, updated)
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
// XPKeywords is UTF-16LE whatever byte order the file uses, because its type is
// BYTE and the bytes are stored as they are written here.
func stockEntries(tags model.Tags) []exifEntry {
	description := exifEntry{tagID: tagImageDescription, tagType: typeASCII, unitSize: 1}
	if title := strings.TrimSpace(tags.Title); len(title) != 0 {
		description.value = append([]byte(title), 0)
	}

	keywords := exifEntry{tagID: tagXPKeywords, tagType: typeByte, unitSize: 1}
	if len(tags.Keywords) != 0 {
		keywords.value = encodeUTF16LE(strings.Join(tags.Keywords, keywordSeparator))
	}

	return []exifEntry{description, keywords}
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
