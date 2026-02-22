package service

import (
	"fmt"
	"os"
	"path/filepath"

	jpegstructure "github.com/dsoprea/go-jpeg-image-structure/v2"
)

type ExifService struct{}

func NewExifService() *ExifService {
	return &ExifService{}
}

func (s *ExifService) GetRating(jpegPath string) (uint16, error) {
	jmp := jpegstructure.NewJpegMediaParser()
	intfc, err := jmp.ParseFile(jpegPath)
	if err != nil {
		return 0, fmt.Errorf("parsing JPEG %s: %w", jpegPath, err)
	}

	sl := intfc.(*jpegstructure.SegmentList)

	rootIfd, _, err := sl.Exif()
	if err != nil {
		//nolint:nilerr // no EXIF data is not an error, just means no rating
		return 0, nil
	}

	results, err := rootIfd.FindTagWithName("Rating")
	if err != nil {
		//nolint:nilerr // tag not found is not an error, just means no rating
		return 0, nil
	}

	if len(results) == 0 {
		return 0, nil
	}

	value, err := results[0].Value()
	if err != nil {
		return 0, fmt.Errorf("reading rating value: %w", err)
	}

	if v, ok := value.([]uint16); ok && len(v) > 0 {
		return v[0], nil
	}

	return 0, nil
}

func (s *ExifService) SetRating(jpegPath string, rating uint16) error {
	jmp := jpegstructure.NewJpegMediaParser()
	intfc, err := jmp.ParseFile(jpegPath)
	if err != nil {
		return fmt.Errorf("parsing JPEG %s: %w", jpegPath, err)
	}

	sl := intfc.(*jpegstructure.SegmentList)

	rootIb, err := sl.ConstructExifBuilder()
	if err != nil {
		return fmt.Errorf("constructing EXIF builder: %w", err)
	}

	if err := rootIb.SetStandardWithName("Rating", []uint16{rating}); err != nil {
		return fmt.Errorf("setting rating: %w", err)
	}

	if err := sl.SetExif(rootIb); err != nil {
		return fmt.Errorf("setting EXIF: %w", err)
	}

	dir := filepath.Dir(jpegPath)
	f, err := os.CreateTemp(dir, "photo-*.jpg")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tempPath := f.Name()

	if err := sl.Write(f); err != nil {
		_ = f.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("writing JPEG: %w", err)
	}
	_ = f.Close()

	if err := os.Rename(tempPath, jpegPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("replacing original file: %w", err)
	}

	return nil
}

func (s *ExifService) ToggleFavorite(jpegPath string) error {
	rating, err := s.GetRating(jpegPath)
	if err != nil {
		return err
	}

	var newRating uint16
	if rating > 0 {
		newRating = 0
	} else {
		newRating = 5
	}

	return s.SetRating(jpegPath, newRating)
}
