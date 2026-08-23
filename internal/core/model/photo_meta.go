package model

import (
	"image"
	"time"
)

type PhotoMeta struct {
	Date     time.Time
	Colors   []ColorLabel
	Favorite bool
	// Ratable says whether the favorite of this photo can be written into its
	// file: any JPEG until the folder scan has read it, then what its packet
	// allows.
	Ratable   bool
	Thumbnail image.Image
}
