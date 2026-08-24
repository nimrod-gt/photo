//go:build !darwin && !windows

package imaging

import (
	"os"
	"time"
)

// Nothing portable reads a birth time here, so the last write is as close as
// the file gets to the day the photo was taken.
func fileCreated(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
