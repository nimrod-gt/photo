package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	exif "github.com/dsoprea/go-exif/v3"
	exifcommon "github.com/dsoprea/go-exif/v3/common"
	jpegstructure "github.com/dsoprea/go-jpeg-image-structure/v2"
)

// IFD entries referencing child IFDs that the library can't map to a known IFD get tag ID 0xffff
const unknownTagID = 0xffff

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

func (s *ExifService) SetRating(jpegPath string, rating uint16) error {
	jmp := jpegstructure.NewJpegMediaParser()
	intfc, err := jmp.ParseFile(jpegPath)
	if err != nil {
		return fmt.Errorf("parsing JPEG %s: %w", jpegPath, err)
	}

	sl, ok := intfc.(*jpegstructure.SegmentList)
	if !ok {
		return fmt.Errorf("unexpected parse result for %s", jpegPath)
	}

	rootIb, err := sl.ConstructExifBuilder()
	if err != nil {
		rootIb, err = constructExifBuilderTolerant(sl)
		if err != nil {
			return fmt.Errorf("constructing EXIF builder: %w", err)
		}
	}

	if err := rootIb.SetStandardWithName("Rating", []uint16{rating}); err != nil {
		return fmt.Errorf("setting rating: %w", err)
	}

	if err := sl.SetExif(rootIb); err != nil {
		return fmt.Errorf("setting EXIF: %w", err)
	}

	origInfo, err := os.Stat(jpegPath)
	if err != nil {
		return fmt.Errorf("stat original file: %w", err)
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
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Chmod(tempPath, origInfo.Mode()); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("setting permissions: %w", err)
	}

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

func constructExifBuilderTolerant(sl *jpegstructure.SegmentList) (*exif.IfdBuilder, error) {
	rootIfd, _, err := sl.Exif()
	if errors.Is(err, exif.ErrNoExif) {
		im := exifcommon.NewIfdMapping()
		if err := exifcommon.LoadStandardIfds(im); err != nil {
			return nil, fmt.Errorf("loading standard IFDs: %w", err)
		}
		ti := exif.NewTagIndex()
		return exif.NewIfdBuilder(im, ti, exifcommon.IfdStandardIfdIdentity, exifcommon.EncodeDefaultByteOrder), nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading EXIF: %w", err)
	}

	return buildChildIfdChainTolerant(rootIfd), nil
}

func buildChildIfdChainTolerant(rootIfd *exif.Ifd) *exif.IfdBuilder {
	var firstIb, lastIb *exif.IfdBuilder
	for thisIfd := rootIfd; thisIfd != nil; thisIfd = thisIfd.NextIfd() {
		newIb := exif.NewIfdBuilderWithExistingIfd(thisIfd)
		if firstIb == nil {
			firstIb = newIb
		} else {
			_ = lastIb.SetNextIb(newIb)
		}
		addTagsTolerant(newIb, thisIfd)
		lastIb = newIb
	}
	return firstIb
}

func addTagsTolerant(ib *exif.IfdBuilder, ifd *exif.Ifd) {
	if data, err := ifd.Thumbnail(); err == nil {
		_ = ib.SetThumbnail(data)
	}

	for i, ite := range ifd.Entries() {
		if ite.IsThumbnailOffset() || ite.IsThumbnailSize() {
			continue
		}

		if len(ite.ChildIfdPath()) > 0 {
			var childIfd *exif.Ifd
			for _, c := range ifd.Children() {
				if c.ParentTagIndex() != i {
					continue
				}
				if c.IfdIdentity().TagId() != unknownTagID && c.IfdIdentity().TagId() != ite.TagId() {
					continue
				}
				childIfd = c
				break
			}
			if childIfd == nil {
				continue
			}
			childIb := buildChildIfdChainTolerant(childIfd)
			_ = ib.AddChildIb(childIb)
			continue
		}

		rawBytes, err := ite.GetRawBytes()
		if err != nil {
			continue
		}
		value := exif.NewIfdBuilderTagValueFromBytes(rawBytes)
		bt := exif.NewBuilderTag(
			ifd.IfdIdentity().UnindexedString(),
			ite.TagId(),
			ite.TagType(),
			value,
			ifd.ByteOrder())
		_ = ib.Add(bt)
	}
}
