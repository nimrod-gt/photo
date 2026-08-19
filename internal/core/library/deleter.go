package library

import (
	"fmt"
	"log"
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
		removeSidecar(photo.RAWPath)
	}

	return nil
}

// The sidecar describes the RAW alone, so it goes with it instead of staying
// behind as an orphan the next scan cannot explain. One that refuses to go is
// not worth failing the delete over: the photo is gone by then, and an error
// here would leave it standing in the list as an entry with no file behind it.
func removeSidecar(rawPath string) {
	sidecar := model.SidecarPath(rawPath)
	if err := os.Remove(sidecar); err != nil && !os.IsNotExist(err) {
		log.Printf("Failed to delete sidecar %s: %v", sidecar, err)
	}
}
