//go:build !darwin && !windows

package service

import "fmt"

func CopyImageToClipboard(path string) error {
	return fmt.Errorf("clipboard copy is not supported on this platform")
}
