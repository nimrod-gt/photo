//go:build !darwin && !windows

package filedate

import (
	"os"
	"time"
)

// Nothing portable reads a birth time here, so Created falls back to the last
// write, which is as close as the file gets to the day the photo was taken.
func birthTime(_ os.FileInfo) (time.Time, bool) {
	return time.Time{}, false
}
