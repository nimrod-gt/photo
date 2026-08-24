//go:build windows

package filedate

import (
	"os"
	"syscall"
	"time"
)

func birthTime(info os.FileInfo) (time.Time, bool) {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(0, data.CreationTime.Nanoseconds()), true
}
