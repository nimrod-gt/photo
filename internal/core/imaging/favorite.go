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
	s.writes.Lock()
	defer s.writes.Unlock()

	original, err := os.ReadFile(jpegPath)
	if err != nil {
		return false, fmt.Errorf("reading %s: %w", jpegPath, err)
	}
	rating, inPacket, err := currentRating(original, jpegPath)
	if err != nil {
		return false, err
	}
	start, end, err := xmpPacketSpan(original)
	if err != nil {
		return false, fmt.Errorf("toggling the favorite of %s: %w", jpegPath, err)
	}
	if start == end {
		return false, fmt.Errorf("%s has no XMP packet to hold a rating", jpegPath)
	}
	if inPacket && !hasRatingSlot(original[start:end]) {
		return false, fmt.Errorf("the rating of %s is written in a form this app cannot update", jpegPath)
	}

	target := favoriteRating
	if rating > 0 {
		target = 0
	}
	packet, ok := packetWithRating(original[start:end], target)
	if !ok {
		return false, fmt.Errorf("the XMP packet of %s is read-only or has no room for a rating", jpegPath)
	}
	if !bytes.Equal(packet, original[start:end]) {
		if err := patchFileKeepingModTime(jpegPath, int64(start), packet); err != nil {
			return false, err
		}
	}
	return target > 0, nil
}

func currentRating(data []byte, source string) (rating int, inPacket bool, err error) {
	sl, err := segmentsFromBytes(data, source)
	if err != nil {
		return 0, false, err
	}
	rating, inPacket, err = xmpRating(sl)
	if err != nil {
		return 0, false, fmt.Errorf("reading the rating of %s: %w", source, err)
	}
	if inPacket {
		return rating, true, nil
	}
	rootIfd, err := exifRootOf(sl)
	if err != nil {
		return 0, false, err
	}
	return exifRating(rootIfd), false, nil
}
