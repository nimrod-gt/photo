package imaging

import (
	"encoding/binary"
	"fmt"
	"slices"
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

// GetStockInfo reads what the files already carry: the EXIF of the JPEG and,
// when the photo has a RAW pair, the sidecar written next to it.
func (s *ExifService) GetStockInfo(photo model.Photo) (StockInfo, error) {
	var info StockInfo
	if photo.IsJPEG() {
		flat, err := flatExifFromFile(photo.ImagePath)
		if err != nil {
			return StockInfo{}, err
		}
		info = stockInfoFromTags(flat)
	}
	if !photo.HasRAW() {
		return info, nil
	}
	// The EXIF is already read at this point, so an unreadable sidecar is
	// reported without throwing away what the JPEG itself carries.
	sidecar, err := ReadSidecar(model.SidecarPath(photo.RAWPath))
	if err != nil {
		return info, err
	}
	info.Tags = fillMissing(info.Tags, sidecar)
	return info, nil
}

func fillMissing(tags, fallback model.Tags) model.Tags {
	if len(strings.TrimSpace(tags.Title)) == 0 {
		tags.Title = fallback.Title
	}
	if len(tags.Keywords) == 0 {
		tags.Keywords = fallback.Keywords
	}
	return tags
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

// The thumbnail IFD repeats the tags of the primary one, and cameras leave its
// copies blank, so it is skipped rather than allowed to overwrite them.
const thumbnailIfdPath = "IFD1"

func stockInfoFromTags(flat []exif.ExifTag) StockInfo {
	var info StockInfo
	dates := make(map[string]string, len(dateTagsByPriority))

	for _, tag := range flat {
		if tag.IfdPath == thumbnailIfdPath {
			continue
		}
		switch {
		case tag.TagName == "ImageDescription":
			info.Tags.Title = strings.TrimSpace(exifString(tag.Value))
		case tag.TagName == "XPKeywords":
			info.Tags.Keywords = parseKeywordTag(tag.Value)
		case slices.Contains(dateTagsByPriority, tag.TagName):
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

func encodeUTF16LE(text string) []byte {
	codes := utf16.Encode([]rune(text + "\x00"))
	data := make([]byte, 0, len(codes)*2)
	for _, code := range codes {
		data = binary.LittleEndian.AppendUint16(data, code)
	}
	return data
}

func decodeUTF16LE(data []byte) string {
	codes := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		codes = append(codes, binary.LittleEndian.Uint16(data[i:]))
	}
	return strings.TrimRight(string(utf16.Decode(codes)), "\x00")
}
