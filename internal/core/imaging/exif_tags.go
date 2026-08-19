package imaging

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"
	"unicode/utf16"

	exif "github.com/dsoprea/go-exif/v3"

	"photo/internal/core/model"
)

const exifDateLayout = "2006:01:02 15:04:05"

var dateTagsByPriority = []string{"DateTimeOriginal", "DateTimeDigitized", "DateTime"}

type StockInfo struct {
	Tags  model.Tags
	Taken time.Time
}

func (s *ExifService) GetStockInfo(jpegPath string) (StockInfo, error) {
	flat, err := flatExifFromFile(jpegPath)
	if err != nil {
		return StockInfo{}, err
	}
	return stockInfoFromTags(flat), nil
}

func flatExifFromFile(jpegPath string) ([]exif.ExifTag, error) {
	sl, err := segmentsFromFile(jpegPath)
	if err != nil {
		return nil, err
	}
	_, rawExif, err := sl.Exif()
	if err != nil {
		//nolint:nilerr // no EXIF data means nothing to read
		return nil, nil
	}
	flat, _, err := exif.GetFlatExifData(rawExif, nil)
	if err != nil {
		return nil, fmt.Errorf("reading EXIF of %s: %w", jpegPath, err)
	}
	return flat, nil
}

func stockInfoFromTags(flat []exif.ExifTag) StockInfo {
	var info StockInfo
	dates := make(map[string]string, len(dateTagsByPriority))

	for _, tag := range flat {
		switch tag.TagName {
		case "ImageDescription":
			info.Tags.Title = strings.TrimSpace(exifString(tag.Value))
		case "XPKeywords":
			info.Tags.Keywords = parseKeywordTag(tag.Value)
		case "DateTimeOriginal", "DateTimeDigitized", "DateTime":
			dates[tag.TagName] = strings.TrimSpace(exifString(tag.Value))
		}
	}

	for _, name := range dateTagsByPriority {
		if taken, err := time.Parse(exifDateLayout, dates[name]); err == nil {
			info.Taken = taken
			break
		}
	}
	return info
}

// XPKeywords is written by Windows as a semicolon-separated list, while stock
// tools tend to write commas; both separators are accepted.
func parseKeywordTag(value any) []string {
	return model.ParseKeywordLine(strings.ReplaceAll(exifString(value), ";", ","))
}

func exifString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimRight(v, "\x00")
	case []byte:
		return decodeUTF16LE(v)
	}
	return ""
}

func decodeUTF16LE(data []byte) string {
	codes := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		codes = append(codes, binary.LittleEndian.Uint16(data[i:]))
	}
	return strings.TrimRight(string(utf16.Decode(codes)), "\x00")
}
