package imaging

import (
	"os"
	"syscall"
	"time"
)

// A photo the camera wrote and the app later rewrote keeps the day it was
// taken, so the file is dated by its birth time and not by its last write.
func fileCreated(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return info.ModTime()
	}
	return time.Unix(st.Birthtimespec.Sec, st.Birthtimespec.Nsec)
}
