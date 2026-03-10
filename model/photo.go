package model

import (
	"os"
	"path/filepath"
	"strings"
)

var rawExtensions = []string{".ARW"}

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

var rawVariants = func() []string {
	seen := make(map[string]struct{})
	var variants []string
	for _, ext := range rawExtensions {
		for _, v := range []string{ext, strings.ToUpper(ext), strings.ToLower(ext)} {
			if _, ok := seen[v]; !ok {
				seen[v] = struct{}{}
				variants = append(variants, v)
			}
		}
	}
	return variants
}()

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
