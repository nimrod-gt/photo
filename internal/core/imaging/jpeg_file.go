package imaging

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	markerStart = 0xff
	markerSOI   = 0xd8
	markerEOI   = 0xd9
	markerSOS   = 0xda
	markerAPP0  = 0xe0
	markerAPP1  = 0xe1

	segmentHeaderSize = 4
	maxSegmentSize    = 0xffff
)

var (
	exifSegmentPrefix = []byte("Exif\x00\x00")
	xmpSegmentPrefix  = []byte(xmpNamespace + "\x00")
)

// Only the EXIF segment is replaced and the rest of the file is copied byte for
// byte, because cameras append a second image (the MPF preview) behind the end
// of the primary one and re-encoding the JPEG would drop it.
func spliceExifSegment(original []byte, start, end int, tiff []byte) ([]byte, error) {
	payload := len(exifSegmentPrefix) + len(tiff)
	if payload+2 > maxSegmentSize {
		return nil, fmt.Errorf("the EXIF needs %d bytes, a JPEG segment holds %d", payload+2, maxSegmentSize)
	}

	segment := make([]byte, 0, segmentHeaderSize+payload)
	segment = append(segment, markerStart, markerAPP1)
	segment = binary.BigEndian.AppendUint16(segment, uint16(payload+2))
	segment = append(segment, exifSegmentPrefix...)
	segment = append(segment, tiff...)

	updated := make([]byte, 0, len(original)-(end-start)+len(segment))
	updated = append(updated, original[:start]...)
	updated = append(updated, segment...)
	return append(updated, original[end:]...), nil
}

type segmentSpan struct {
	marker     byte
	start, end int
}

func (s segmentSpan) payload(data []byte) []byte {
	return data[s.start+segmentHeaderSize : s.end]
}

// segmentSpans lists the segments in front of the image data. The scan stops at
// the first SOS, so the second image cameras append behind the primary one is
// never reached.
func segmentSpans(data []byte) ([]segmentSpan, error) {
	if len(data) < segmentHeaderSize || data[0] != markerStart || data[1] != markerSOI {
		return nil, errors.New("not a JPEG")
	}
	var spans []segmentSpan
	for pos := 2; pos+segmentHeaderSize <= len(data); {
		marker := data[pos+1]
		if data[pos] != markerStart || marker == markerSOS || marker == markerEOI {
			break
		}
		length := int(binary.BigEndian.Uint16(data[pos+2:]))
		if length < 2 || pos+2+length > len(data) {
			return nil, errors.New("truncated JPEG segment")
		}
		spans = append(spans, segmentSpan{marker: marker, start: pos, end: pos + 2 + length})
		pos += 2 + length
	}
	return spans, nil
}

// exifSegmentSpan reports where the EXIF segment sits. A file without one gets
// an empty span where the segment belongs: right behind the start marker, or
// behind the JFIF header when the file opens with one, because JFIF claims the
// first segment for itself.
func exifSegmentSpan(data []byte) (start, end int, err error) {
	spans, err := segmentSpans(data)
	if err != nil {
		return 0, 0, err
	}
	insertAt := 2
	for _, span := range spans {
		if span.marker == markerAPP1 && bytes.HasPrefix(span.payload(data), exifSegmentPrefix) {
			return span.start, span.end, nil
		}
		if span.marker == markerAPP0 && insertAt == span.start {
			insertAt = span.end
		}
	}
	return insertAt, insertAt, nil
}

// xmpPacketSpan reports where the XMP packet sits: the bytes behind the
// namespace identifier up to the end of its segment. An empty span means the
// file carries none.
func xmpPacketSpan(data []byte) (start, end int, err error) {
	spans, err := segmentSpans(data)
	if err != nil {
		return 0, 0, err
	}
	for _, span := range spans {
		if span.marker == markerAPP1 && bytes.HasPrefix(span.payload(data), xmpSegmentPrefix) {
			return span.start + segmentHeaderSize + len(xmpSegmentPrefix), span.end, nil
		}
	}
	return 0, 0, nil
}

// The file is replaced through a temp file in its own directory, so a failed
// write never leaves a half-written original behind.
func replaceFile(path string, data []byte) error {
	tmpPath, err := writeTempFile(path, data)
	if err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming %s: %w", tmpPath, err)
	}
	return nil
}

func replaceFileKeepingModTime(path string, data []byte) error {
	return keepingModTime(path, func() error { return replaceFile(path, data) })
}

// The bytes are written over their own place in the file, which keeps its size,
// its blocks and its directory entry: a camera keeps a database of the files it
// wrote and refuses to display a photo whose file no longer matches it until
// the user rebuilds that database. Only the modification time moves, and it is
// put back for the reasons below.
func patchFileKeepingModTime(path string, offset int64, data []byte) error {
	return keepingModTime(path, func() error { return patchFile(path, offset, data) })
}

func patchFile(path string, offset int64, data []byte) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}
	if err := writeAtAndClose(file, offset, data); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// Writing tags changes what the photo says about itself, not when it was taken,
// so its modification time is put back afterwards. A fresh one would move the
// file under the time sort, out of the group the Today button selects, and
// shift the date the tags dialog offers the next time it opens.
func keepingModTime(path string, write func() error) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	if err := write(); err != nil {
		return err
	}
	return os.Chtimes(path, info.ModTime(), info.ModTime())
}

func writeTempFile(path string, data []byte) (string, error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return "", fmt.Errorf("creating a temp file for %s: %w", path, err)
	}
	tmpPath := tmp.Name()
	if err := writeAndClose(tmp, data); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("writing %s: %w", tmpPath, err)
	}
	if err := os.Chmod(tmpPath, permissionsOf(path)); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("setting the permissions of %s: %w", tmpPath, err)
	}
	return tmpPath, nil
}

// The data is flushed before the rename, because the rename only publishes the
// directory entry: without the sync a crash can leave a truncated file standing
// where the original photo used to be.
func writeAndClose(file *os.File, data []byte) error {
	return writeAtAndClose(file, 0, data)
}

func writeAtAndClose(file *os.File, offset int64, data []byte) error {
	if _, err := file.WriteAt(data, offset); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func permissionsOf(path string) os.FileMode {
	info, err := os.Stat(path)
	if err != nil {
		return defaultFilePerm
	}
	return info.Mode().Perm()
}
