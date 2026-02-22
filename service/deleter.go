package service

import (
	"fmt"
	"os"

	"photo/model"
)

type Deleter struct{}

func NewDeleter() *Deleter {
	return &Deleter{}
}

func (d *Deleter) Delete(photo model.Photo) error {
	if err := os.Remove(photo.JPEGPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting JPEG %s: %w", photo.JPEGPath, err)
	}

	if photo.HasRAW() {
		if err := os.Remove(photo.RAWPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("deleting RAW %s: %w", photo.RAWPath, err)
		}
	}

	return nil
}
