package imaging

import (
	"os"
	"syscall"
	"time"
)

// A photo the camera wrote and the app later rewrote keeps the day it was
// taken, so the file is dated by its creation time and not by its last write.
func fileCreated(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return info.ModTime()
	}
	return time.Unix(0, data.CreationTime.Nanoseconds())
}
