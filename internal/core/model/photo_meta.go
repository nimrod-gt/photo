package model

import (
	"image"
	"time"
)

type PhotoMeta struct {
	Date      time.Time
	Colors    []ColorLabel
	Favorite  bool
	Thumbnail image.Image
}
