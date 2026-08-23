package imaging

import (
	"bytes"
	"fmt"
	"os"
)

const favoriteRating = 5

// ToggleFavorite flips the rating of the JPEG between five stars and none the
// way the camera does it: in place, inside the XMP packet, so the file keeps
// its size, its layout and its directory entry and the camera keeps showing
// it. A JPEG whose packet cannot take the rating is reported and left alone
// rather than rewritten around it.
func (s *ExifService) ToggleFavorite(jpegPath string) (favorite bool, err error) {
	s.access.Lock()
	defer s.access.Unlock()

	original, err := os.ReadFile(jpegPath)
	if err != nil {
		return false, fmt.Errorf("reading %s: %w", jpegPath, err)
	}
	start, end, err := xmpPacketSpan(original)
	if err != nil {
		return false, fmt.Errorf("toggling the favorite of %s: %w", jpegPath, err)
	}
	if start == end {
		return false, fmt.Errorf("%s has no XMP packet to hold a rating", jpegPath)
	}
	rating, err := currentRating(original, jpegPath)
	if err != nil {
		return false, err
	}

	target := favoriteRating
	if rating > 0 {
		target = 0
	}
	packet, ok := packetWithRating(original[start:end], target)
	if !ok {
		return false, fmt.Errorf("the XMP packet of %s cannot take a rating: it is read-only, has no room, "+
			"or holds one written in a form this app does not rewrite", jpegPath)
	}
	if !bytes.Equal(packet, original[start:end]) {
		if err := patchPacket(jpegPath, start, end, packet); err != nil {
			return false, err
		}
	}
	return target > 0, nil
}

// A packet the parser rejects still leaves the EXIF rating to go by, which is
// what the folder scan shows for it; failing here instead would refuse to toggle
// a photo that is showing a star. Writing into such a packet is refused further
// down, where the refusal can be reported for what it is.
func currentRating(data []byte, source string) (int, error) {
	sl, err := segmentsFromBytes(data, source)
	if err != nil {
		return 0, err
	}
	rootIfd, err := exifRootOf(sl)
	if err != nil {
		return 0, err
	}
	rating, _ := ratingOf(sl, rootIfd)
	return rating, nil
}
