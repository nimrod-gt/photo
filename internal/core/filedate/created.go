package filedate

import (
	"os"
	"time"
)

// A photo the camera wrote and the app later rewrote keeps the day it was
// taken, so the file is dated by its birth time and not by its last write.
func Created(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	if birth, ok := birthTime(info); ok {
		return birth
	}
	return info.ModTime()
}
