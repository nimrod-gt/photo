package service

import (
	"context"
	"image"
	"log"

	"photo/model"
)

type MetadataLoader struct {
	exif *ExifService
}

func NewMetadataLoader(exif *ExifService) *MetadataLoader {
	return &MetadataLoader{exif: exif}
}

func (l *MetadataLoader) LoadAsync(
	ctx context.Context,
	photos []model.Photo,
	onLoaded func(index int, thumbnail image.Image, favorite bool),
	onComplete func(),
) {
	go func() {
		for i, photo := range photos {
			if ctx.Err() != nil {
				return
			}
			if !photo.IsJPEG() {
				onLoaded(i, nil, false)
				continue
			}
			thumbnail, rating, err := l.exif.GetPhotoInfo(photo.ImagePath)
			if err != nil {
				log.Printf("Failed to read EXIF for %s: %v", photo.Name, err)
				onLoaded(i, nil, false)
				continue
			}
			if ctx.Err() != nil {
				return
			}
			onLoaded(i, thumbnail, rating > 0)
		}
		if ctx.Err() == nil && onComplete != nil {
			onComplete()
		}
	}()
}
