package imaging

import (
	"fmt"
	"image"
	"sync"

	exif "github.com/dsoprea/go-exif/v3"
	jpegstructure "github.com/dsoprea/go-jpeg-image-structure/v2"
)

type ExifService struct {
	// Every writer reads the whole file and patches what it read back, so two of
	// them at once would each undo the other's change. Readers are held off as
	// well: a folder scan runs on as many goroutines as the machine has cores,
	// and one landing in the middle of a patch would read a packet that is half
	// old and half new and report the photo as unrated.
	access sync.RWMutex
}

func NewExifService() *ExifService {
	return &ExifService{}
}

func segmentsFromFile(jpegPath string) (*jpegstructure.SegmentList, error) {
	jmp := jpegstructure.NewJpegMediaParser()
	intfc, err := jmp.ParseFile(jpegPath)
	return segmentsOf(intfc, err, jpegPath)
}

func segmentsFromBytes(data []byte, source string) (*jpegstructure.SegmentList, error) {
	jmp := jpegstructure.NewJpegMediaParser()
	intfc, err := jmp.ParseBytes(data)
	return segmentsOf(intfc, err, source)
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

func exifRootOf(sl *jpegstructure.SegmentList) (*exif.Ifd, error) {
	rootIfd, _, err := sl.Exif()
	if err != nil {
		//nolint:nilerr // no EXIF data means nothing to read
		return nil, nil
	}
	return rootIfd, nil
}

// the orientation belongs to the parent IFD0 rather than to the thumbnail, so
// it is applied here and never leaves: the main image is rotated by the decoder
// off the file's own tag
func (s *ExifService) GetPhotoInfo(jpegPath string) (thumbnail image.Image, rating int, err error) {
	s.access.RLock()
	defer s.access.RUnlock()

	sl, err := segmentsFromFile(jpegPath)
	if err != nil {
		return nil, 0, err
	}
	rootIfd, err := exifRootOf(sl)
	if err != nil {
		return nil, 0, err
	}
	if rootIfd != nil {
		thumbnail = extractThumbnail(rootIfd)
		if thumbnail != nil {
			thumbnail = applyOrientation(thumbnail, int(ifdUint16(rootIfd, "Orientation", 1)))
		}
	}
	// A packet the parser rejects still leaves the EXIF rating to show.
	rating, _ = ratingOf(sl, rootIfd)
	return thumbnail, rating, nil
}

// The camera and Lightroom keep the rating in the XMP packet and only older
// tools in the EXIF, so the packet wins whenever it says anything, zero
// included: that is what the camera shows.
func ratingOf(sl *jpegstructure.SegmentList, rootIfd *exif.Ifd) (int, error) {
	rating, found, err := xmpRating(sl)
	if found {
		return rating, nil
	}
	return exifRating(rootIfd), err
}

func xmpRating(sl *jpegstructure.SegmentList) (rating int, found bool, err error) {
	packet := xmpPacketOf(sl)
	if packet == nil {
		return 0, false, nil
	}
	parsed, err := parseSidecar(packet)
	if err != nil {
		return 0, false, err
	}
	return parsed.rating, parsed.rated, nil
}

func exifRating(rootIfd *exif.Ifd) int {
	if rootIfd == nil {
		return 0
	}
	return int(ifdUint16(rootIfd, "Rating", 0))
}

func extractThumbnail(rootIfd *exif.Ifd) image.Image {
	if nextIfd := rootIfd.NextIfd(); nextIfd != nil {
		if img := decodeThumbnailOf(nextIfd); img != nil {
			return img
		}
	}
	return decodeThumbnailOf(rootIfd)
}

func decodeThumbnailOf(ifd *exif.Ifd) image.Image {
	thumbData, err := ifd.Thumbnail()
	if err != nil || len(thumbData) == 0 {
		return nil
	}
	img, err := decodeJPEGThumbnail(thumbData)
	if err != nil {
		return nil
	}
	return img
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
