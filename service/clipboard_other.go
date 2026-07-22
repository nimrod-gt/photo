//go:build !darwin && !windows

package service

import "errors"

func CopyImageToClipboard(path string) error {
	return errors.New("clipboard copy is not supported on this platform")
}
