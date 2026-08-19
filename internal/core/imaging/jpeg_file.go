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

var exifSegmentPrefix = []byte("Exif\x00\x00")

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

// exifSegmentSpan reports where the EXIF segment sits. A file without one gets
// an empty span where the segment belongs: right behind the start marker, or
// behind the JFIF header when the file opens with one, because JFIF claims the
// first segment for itself.
func exifSegmentSpan(data []byte) (start, end int, err error) {
	if len(data) < segmentHeaderSize || data[0] != markerStart || data[1] != markerSOI {
		return 0, 0, errors.New("not a JPEG")
	}
	insertAt := 2
	for pos := 2; pos+segmentHeaderSize <= len(data); {
		marker := data[pos+1]
		if data[pos] != markerStart || marker == markerSOS || marker == markerEOI {
			break
		}
		length := int(binary.BigEndian.Uint16(data[pos+2:]))
		if length < 2 || pos+2+length > len(data) {
			return 0, 0, errors.New("truncated JPEG segment")
		}
		segmentEnd := pos + 2 + length
		if marker == markerAPP1 && bytes.HasPrefix(data[pos+segmentHeaderSize:segmentEnd], exifSegmentPrefix) {
			return pos, segmentEnd, nil
		}
		if marker == markerAPP0 && insertAt == pos {
			insertAt = segmentEnd
		}
		pos = segmentEnd
	}
	return insertAt, insertAt, nil
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

// Writing tags changes what the photo says about itself, not when it was taken,
// so its modification time is put back afterwards. A fresh one would move the
// file under the time sort, out of the group the Today button selects, and
// shift the date the tags dialog offers the next time it opens.
func replaceFileKeepingModTime(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	if err := replaceFile(path, data); err != nil {
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
	if _, err := file.Write(data); err != nil {
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
