//go:build !darwin && !windows

package clipboard

import "errors"

func CopyImage(path string) error {
	return errors.New("clipboard copy is not supported on this platform")
}
