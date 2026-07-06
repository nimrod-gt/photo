package service

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

func (s *ExifService) GetRating(jpegPath string) (uint16, error) {
	return s.getUint16Tag(jpegPath, "Rating", 0)
}

func (s *ExifService) GetOrientation(jpegPath string) (uint16, error) {
	return s.getUint16Tag(jpegPath, "Orientation", 1)
}

func (s *ExifService) getUint16Tag(jpegPath, tagName string, defaultVal uint16) (uint16, error) {
	jmp := jpegstructure.NewJpegMediaParser()
	intfc, err := jmp.ParseFile(jpegPath)
	if err != nil {
		return defaultVal, fmt.Errorf("parsing JPEG %s: %w", jpegPath, err)
	}

	sl, ok := intfc.(*jpegstructure.SegmentList)
	if !ok {
		return defaultVal, fmt.Errorf("unexpected parse result for %s", jpegPath)
	}

	rootIfd, _, err := sl.Exif()
	if err != nil {
		//nolint:nilerr // no EXIF data means default value
		return defaultVal, nil
	}

	results, err := rootIfd.FindTagWithName(tagName)
	if err != nil {
		//nolint:nilerr // tag not found means default value
		return defaultVal, nil
	}

	if len(results) == 0 {
		return defaultVal, nil
	}

	value, err := results[0].Value()
	if err != nil {
		return defaultVal, fmt.Errorf("reading %s value: %w", tagName, err)
	}

	if v, ok := value.([]uint16); ok && len(v) > 0 {
		return v[0], nil
	}

	return defaultVal, nil
}

func (s *ExifService) GetPhotoInfo(jpegPath string) (thumbnail image.Image, rating, orientation uint16, err error) {
	jmp := jpegstructure.NewJpegMediaParser()
	intfc, err := jmp.ParseFile(jpegPath)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("parsing JPEG %s: %w", jpegPath, err)
	}

	sl, ok := intfc.(*jpegstructure.SegmentList)
	if !ok {
		return nil, 0, 0, fmt.Errorf("unexpected parse result for %s", jpegPath)
	}

	rootIfd, _, err := sl.Exif()
	if err != nil {
		//nolint:nilerr // no EXIF data means no metadata to extract
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
