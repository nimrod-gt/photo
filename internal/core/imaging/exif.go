package imaging

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"

	exif "github.com/dsoprea/go-exif/v3"
	jpegstructure "github.com/dsoprea/go-jpeg-image-structure/v2"
)

type ExifService struct{}

func NewExifService() *ExifService {
	return &ExifService{}
}

func segmentsFromFile(jpegPath string) (*jpegstructure.SegmentList, error) {
	jmp := jpegstructure.NewJpegMediaParser()
	intfc, err := jmp.ParseFile(jpegPath)
	return segmentsOf(intfc, err, jpegPath)
}

func segmentsFromBytes(data []byte) (*jpegstructure.SegmentList, error) {
	jmp := jpegstructure.NewJpegMediaParser()
	intfc, err := jmp.ParseBytes(data)
	return segmentsOf(intfc, err, "buffer")
}

func segmentsOf(intfc any, parseErr error, source string) (*jpegstructure.SegmentList, error) {
	if parseErr != nil {
		return nil, fmt.Errorf("parsing JPEG %s: %w", source, parseErr)
	}
	sl, ok := intfc.(*jpegstructure.SegmentList)
	if !ok {
		return nil, fmt.Errorf("unexpected parse result for %s", source)
	}
	return sl, nil
}

// returns (nil, nil) when the JPEG parses but carries no EXIF data
func exifRootFromFile(jpegPath string) (*exif.Ifd, error) {
	sl, err := segmentsFromFile(jpegPath)
	if err != nil {
		return nil, err
	}
	return exifRootOf(sl)
}

func exifRootFromBytes(data []byte) (*exif.Ifd, error) {
	sl, err := segmentsFromBytes(data)
	if err != nil {
		return nil, err
	}
	return exifRootOf(sl)
}

func exifRootOf(sl *jpegstructure.SegmentList) (*exif.Ifd, error) {
	rootIfd, _, err := sl.Exif()
	if err != nil {
		//nolint:nilerr // no EXIF data means nothing to read
		return nil, nil
	}
	return rootIfd, nil
}

func (s *ExifService) GetPhotoInfo(jpegPath string) (thumbnail image.Image, rating, orientation uint16, err error) {
	rootIfd, err := exifRootFromFile(jpegPath)
	if err != nil {
		return nil, 0, 0, err
	}
	if rootIfd == nil {
		return nil, 0, 1, nil
	}

	orientation = ifdUint16(rootIfd, "Orientation", 1)

	thumbnail = extractThumbnail(rootIfd)
	if thumbnail != nil {
		thumbnail = applyOrientation(thumbnail, orientation)
	}

	rating = ifdUint16(rootIfd, "Rating", 0)
	return thumbnail, rating, orientation, nil
}

func extractThumbnail(rootIfd *exif.Ifd) image.Image {
	if nextIfd := rootIfd.NextIfd(); nextIfd != nil {
		if thumbData, err := nextIfd.Thumbnail(); err == nil && len(thumbData) > 0 {
			if img, err := jpeg.Decode(bytes.NewReader(thumbData)); err == nil {
				return img
			}
		}
	}
	if thumbData, err := rootIfd.Thumbnail(); err == nil && len(thumbData) > 0 {
		if img, err := jpeg.Decode(bytes.NewReader(thumbData)); err == nil {
			return img
		}
	}
	return nil
}

func ifdUint16(ifd *exif.Ifd, tagName string, defaultVal uint16) uint16 {
	results, err := ifd.FindTagWithName(tagName)
	if err != nil || len(results) == 0 {
		return defaultVal
	}
	value, err := results[0].Value()
	if err != nil {
		return defaultVal
	}
	if v, ok := value.([]uint16); ok && len(v) > 0 {
		return v[0]
	}
	return defaultVal
}
