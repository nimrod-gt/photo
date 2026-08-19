package library

import (
	"fmt"
	"os"

	"photo/internal/core/model"
)

type Deleter struct{}

func NewDeleter() *Deleter {
	return &Deleter{}
}

func (d *Deleter) Delete(photo model.Photo) error {
	return d.DeleteWithOption(photo, true)
}

func (d *Deleter) DeleteWithOption(photo model.Photo, includeRAW bool) error {
	if err := os.Remove(photo.ImagePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting image %s: %w", photo.ImagePath, err)
	}

	if includeRAW && photo.HasRAW() {
		if err := os.Remove(photo.RAWPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("deleting RAW %s: %w", photo.RAWPath, err)
		}
		// The sidecar describes the RAW alone, so it goes with it instead of
		// staying behind as an orphan the next scan cannot explain.
		sidecar := model.SidecarPath(photo.RAWPath)
		if err := os.Remove(sidecar); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("deleting sidecar %s: %w", sidecar, err)
		}
	}

	return nil
}
