package model

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

var supportedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
}

type Photo struct {
	ImagePath string
	RAWPath   string
	Name      string
	ModTime   time.Time
}

func NewPhoto(imagePath string) Photo {
	return NewPhotoWithExists(imagePath, func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	})
}

func NewPhotoWithExists(imagePath string, exists func(string) bool) Photo {
	p := Photo{
		ImagePath: imagePath,
		Name:      filepath.Base(imagePath),
	}
	if p.IsJPEG() {
		p.RAWPath = findRAWPair(imagePath, exists)
	}
	return p
}

func IsSupportedImage(ext string) bool {
	return supportedExtensions[strings.ToLower(ext)]
}

func (p Photo) IsJPEG() bool {
	ext := strings.ToLower(filepath.Ext(p.ImagePath))
	return ext == ".jpg" || ext == ".jpeg"
}

func (p Photo) HasRAW() bool {
	return len(p.RAWPath) != 0
}

var rawVariants = []string{".ARW", ".arw"}

func findRAWPair(jpegPath string, exists func(string) bool) string {
	ext := filepath.Ext(jpegPath)
	base := strings.TrimSuffix(jpegPath, ext)
	for _, rawExt := range rawVariants {
		candidate := base + rawExt
		if candidate == jpegPath {
			continue
		}
		if exists(candidate) {
			return candidate
		}
	}
	return ""
}

const sidecarExt = ".xmp"

// The sidecar is where the tags of a photo live: a RAW cannot carry EXIF
// written by us, and a JPEG is only rewritten on demand. It is named after the
// RAW when there is a pair - the convention Lightroom and Bridge use - and
// after the image itself when there is none.
func (p Photo) SidecarPath() string {
	if p.HasRAW() {
		return SidecarPath(p.RAWPath)
	}
	return SidecarPath(p.ImagePath)
}

// SidecarPath swaps the extension of a file for the sidecar one; the caller
// decides which file the sidecar belongs to.
func SidecarPath(path string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + sidecarExt
}
