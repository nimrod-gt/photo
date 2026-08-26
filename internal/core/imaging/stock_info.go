package imaging

import (
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf16"

	exif "github.com/dsoprea/go-exif/v3"
	jpegstructure "github.com/dsoprea/go-jpeg-image-structure/v2"

	"photo/internal/core/filedate"
	"photo/internal/core/model"
)

const exifDateLayout = "2006:01:02 15:04:05"

var dateTagsByPriority = []string{"DateTimeOriginal", "DateTimeDigitized", "DateTime"}

type StockInfo struct {
	Tags  model.Tags
	Taken time.Time
	// complete says the tags need no further reading: the XMP sidecar of a RAW
	// pair is already folded in, or the app itself is the one that wrote them.
	// Tags read out of a JPEG alone are not complete, and the sidecar would
	// otherwise overwrite what was generated for the photo but not saved yet.
	complete bool
}

// A photo whose EXIF carries no date is dated by the file it lives in: the time
// it was created, not the time it was last written, because writing tags into a
// JPEG rewrites the file and must not move the photo's date.
func withFileDate(info StockInfo, path string) StockInfo {
	if info.Taken.IsZero() {
		info.Taken = filedate.Created(path)
	}
	return info
}

// GetStockInfo reads what the files already carry: the XMP packet and the EXIF
// of the JPEG and, when the photo has a RAW pair, the sidecar written next to
// it.
func (s *ExifService) GetStockInfo(photo model.Photo) (StockInfo, error) {
	s.access.RLock()
	defer s.access.RUnlock()

	var info StockInfo
	var jpegErr error
	if photo.IsJPEG() {
		info, jpegErr = jpegStockInfo(photo.ImagePath)
	}
	info = withFileDate(info, photo.ImagePath)
	// The sidecar is read whatever the JPEG had to say about itself: it is the
	// store the dialog writes on its own, and a dialog seeded without it would
	// overwrite the tags it holds with the first save.
	info, err := mergedWithSidecar(info, photo)
	if err != nil {
		return info, errors.Join(jpegErr, err)
	}
	return info, jpegErr
}

// The sidecar holds the newer tags whenever the two disagree; the tags read
// out of the JPEG only fill what it lacks. A sidecar that cannot be read is
// reported, with the tags returned as they were.
func mergedWithSidecar(info StockInfo, photo model.Photo) (StockInfo, error) {
	if !photo.HasRAW() {
		return info, nil
	}
	sidecar, err := ReadSidecar(model.SidecarPath(photo.RAWPath))
	if err != nil {
		return info, err
	}
	info.Tags = FillMissing(sidecar, info.Tags)
	return info, nil
}

// The packet is where the tags are written now, so it wins over the EXIF,
// which only fills what the packet lacks: the tags an earlier version of the
// app wrote there, or what another tool left. The date comes from the EXIF
// alone. A packet that cannot be parsed is reported with the EXIF tags kept.
func jpegStockInfo(jpegPath string) (StockInfo, error) {
	sl, err := segmentsFromFile(jpegPath)
	if err != nil {
		return StockInfo{}, err
	}
	return stockInfoFromSegments(sl, jpegPath)
}

func stockInfoFromSegments(sl *jpegstructure.SegmentList, source string) (StockInfo, error) {
	flat, err := flatExifOf(sl, source)
	if err != nil {
		return StockInfo{}, err
	}
	info := stockInfoFromTags(flat)
	packet := xmpPacketOf(sl)
	if packet == nil {
		return info, nil
	}
	parsed, err := parseSidecar(packet)
	if err != nil {
		return info, fmt.Errorf("parsing the XMP of %s: %w", source, err)
	}
	info.Tags = FillMissing(parsed.tags(), info.Tags)
	return info, nil
}

// FillMissing completes one set of tags from another: a field the newer set
// leaves empty keeps whatever the older one had for it, level by level.
func FillMissing(tags, fallback model.Tags) model.Tags {
	tags.Title = orFallback(tags.Title, fallback.Title)
	if len(tags.Keywords) == 0 {
		tags.Keywords = fallback.Keywords
	}
	tags.Place = fillMissingPlace(tags.Place, fallback.Place)
	tags.Concept = orFallback(tags.Concept, fallback.Concept)
	return tags
}

// Level by level: a sidecar that names the country but not the city is no reason
// to drop the city the packet of the JPEG carries.
func fillMissingPlace(place, fallback model.Place) model.Place {
	return model.Place{
		Location: orFallback(place.Location, fallback.Location),
		City:     orFallback(place.City, fallback.City),
		State:    orFallback(place.State, fallback.State),
		Country:  orFallback(place.Country, fallback.Country),
	}
}

func orFallback(value, fallback string) string {
	if len(strings.TrimSpace(value)) == 0 {
		return fallback
	}
	return value
}

func xmpPacketOf(sl *jpegstructure.SegmentList) []byte {
	_, segment, err := sl.FindXmp()
	if err != nil {
		return nil
	}
	return segment.Data[len(xmpSegmentPrefix):]
}

func flatExifOf(sl *jpegstructure.SegmentList, jpegPath string) ([]exif.ExifTag, error) {
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
	var xpTitle string
	dates := make(map[string]string, len(dateTagsByPriority))

	for _, tag := range flat {
		if tag.IfdPath == thumbnailIfdPath {
			continue
		}
		switch {
		case tag.TagName == "ImageDescription":
			info.Tags.Title = strings.TrimSpace(exifString(tag.Value))
		case tag.TagName == "XPTitle":
			xpTitle = strings.TrimSpace(exifString(tag.Value))
		case tag.TagName == "XPKeywords":
			info.Tags.Keywords = parseKeywordTag(tag.Value)
		case slices.Contains(dateTagsByPriority, tag.TagName):
			dates[tag.TagName] = strings.TrimSpace(exifString(tag.Value))
		}
	}

	// XPTitle is UTF-16 and carries the titles ImageDescription cannot spell, so
	// it wins wherever a file has both.
	if len(xpTitle) != 0 {
		info.Tags.Title = xpTitle
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
