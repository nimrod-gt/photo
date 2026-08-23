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

// A block that cannot be read and no block at all are the same nothing here:
// the caller still has the XMP packet to read the rating from, and reporting a
// broken EXIF as a failure would take that away along with the thumbnail.
func exifRootOf(sl *jpegstructure.SegmentList) *exif.Ifd {
	rootIfd, _, err := sl.Exif()
	if err != nil {
		return nil
	}
	return rootIfd
}

type PhotoInfo struct {
	Thumbnail image.Image
	Rating    int
	// Ratable says whether ToggleFavorite can write into this file, so the
	// button that calls it is enabled only where a press can succeed.
	Ratable bool
}

// the orientation belongs to the parent IFD0 rather than to the thumbnail, so
// it is applied here and never leaves: the main image is rotated by the decoder
// off the file's own tag
func (s *ExifService) GetPhotoInfo(jpegPath string) (PhotoInfo, error) {
	s.access.RLock()
	defer s.access.RUnlock()

	sl, err := segmentsFromFile(jpegPath)
	if err != nil {
		return PhotoInfo{}, err
	}
	var info PhotoInfo
	rootIfd := exifRootOf(sl)
	if rootIfd != nil {
		info.Thumbnail = extractThumbnail(rootIfd)
		if info.Thumbnail != nil {
			info.Thumbnail = applyOrientation(info.Thumbnail, int(ifdUint16(rootIfd, "Orientation", 1)))
		}
	}
	// A packet the parser rejects still leaves the EXIF rating to show.
	info.Rating, _ = ratingOf(sl, rootIfd)
	info.Ratable = packetTakesRating(xmpPacketOf(sl))
	return info, nil
}

// The toggle writes five or zero, and a packet that takes one takes the other
// just the same: the digit changes where it stands, or an element is appended
// behind whatever the room allows.
func packetTakesRating(packet []byte) bool {
	_, ok := packetWithRating(packet, favoriteRating)
	return ok
}

// The camera and Lightroom keep the rating in the XMP packet and only older
// tools in the EXIF, so the packet wins whenever it says anything, zero
// included: that is what the camera shows.
func ratingOf(sl *jpegstructure.SegmentList, rootIfd *exif.Ifd) (int, error) {
	rating, found, err := packetRating(xmpPacketOf(sl))
	if found {
		return rating, nil
	}
	return exifRating(rootIfd), err
}

func packetRating(packet []byte) (rating int, found bool, err error) {
	if len(packet) == 0 {
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
