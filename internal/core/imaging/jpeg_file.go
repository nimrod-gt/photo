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

// patchPacket writes a rewritten XMP packet back over the bytes it was read
// from. Its length is checked rather than trusted: a packet of another size
// would silently run over the segments behind it.
//
// The bytes are written over their own place in the file, which keeps its size,
// its blocks and its directory entry: a camera keeps a database of the files it
// wrote and refuses to display a photo whose file no longer matches it until
// the user rebuilds that database. The write is not atomic, unlike replaceFile:
// a fault in the middle of it leaves the packet half old and half new in the
// only copy of the photo there is, so the bytes that stood there are put back
// before the failure is reported. Putting them back is best-effort - the disk
// has just refused a write of the same size at the same offset - and its own
// failure is reported alongside the first. A file that could not even be
// opened is untouched and nothing is restored.
func patchPacket(path string, start int, packet, previous []byte) error {
	if len(packet) != len(previous) {
		return fmt.Errorf("the XMP packet of %s would change size from %d to %d bytes",
			path, len(previous), len(packet))
	}
	return keepingModTime(path, func() (err error) {
		file, closeFile, err := openForPatching(path)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, closeFile()) }()

		if err := writeAtSynced(file, int64(start), packet); err != nil {
			if restore := writeAtSynced(file, int64(start), previous); restore != nil {
				return errors.Join(err, fmt.Errorf("restoring the XMP packet of %s: %w", path, restore))
			}
			return err
		}
		return nil
	})
}

// A photo copied off a locked card, or marked read-only by hand, still takes
// the write the user asked for, the way replaceFile always has: the write bit
// is lifted for the duration of the patch and put back with the file closed.
func openForPatching(path string) (file *os.File, closeFile func() error, err error) {
	file, err = os.OpenFile(path, os.O_RDWR, 0)
	if err == nil {
		return file, file.Close, nil
	}
	if !errors.Is(err, os.ErrPermission) {
		return nil, nil, fmt.Errorf("opening %s: %w", path, err)
	}
	mode := permissionsOf(path)
	if err := os.Chmod(path, mode|0o200); err != nil {
		return nil, nil, fmt.Errorf("opening %s: %w", path, err)
	}
	file, err = os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		_ = os.Chmod(path, mode)
		return nil, nil, fmt.Errorf("opening %s: %w", path, err)
	}
	return file, func() error {
		return errors.Join(file.Close(), os.Chmod(path, mode))
	}, nil
}

// The data is flushed before the caller moves on, because a rename only
// publishes the directory entry: without the sync a crash can leave a truncated
// file standing where the original photo used to be.
func writeAtSynced(file *os.File, offset int64, data []byte) error {
	if _, err := file.WriteAt(data, offset); err != nil {
		return fmt.Errorf("writing %s: %w", file.Name(), err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("writing %s: %w", file.Name(), err)
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
	// The file is already written at this point, so a mount that refuses to move
	// its timestamps is not worth failing over: the caller would report a change
	// that is on disk as a failure and leave the UI showing the opposite of it.
	_ = os.Chtimes(path, info.ModTime(), info.ModTime())
	return nil
}

func writeTempFile(path string, data []byte) (string, error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return "", fmt.Errorf("creating a temp file for %s: %w", path, err)
	}
	tmpPath := tmp.Name()
	if err := writeAndClose(tmp, data); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	if err := os.Chmod(tmpPath, permissionsOf(path)); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("setting the permissions of %s: %w", tmpPath, err)
	}
	return tmpPath, nil
}

func writeAndClose(file *os.File, data []byte) error {
	if err := writeAtSynced(file, 0, data); err != nil {
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
