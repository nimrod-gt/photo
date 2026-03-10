package model

import (
	"os"
	"path/filepath"
	"strings"
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
}

func NewPhoto(imagePath string) Photo {
	p := Photo{
		ImagePath: imagePath,
		Name:      filepath.Base(imagePath),
	}
	if p.IsJPEG() {
		p.RAWPath = findRAWPair(imagePath)
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

func findRAWPair(jpegPath string) string {
	ext := filepath.Ext(jpegPath)
	base := strings.TrimSuffix(jpegPath, ext)
	for _, rawExt := range rawVariants {
		candidate := base + rawExt
		if candidate == jpegPath {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}
