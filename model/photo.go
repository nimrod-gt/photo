package model

import (
	"os"
	"path/filepath"
	"strings"
)

var rawExtensions = []string{".ARW"}

type Photo struct {
	JPEGPath string
	RAWPath  string
	Name     string
}

func NewPhoto(jpegPath string) Photo {
	return Photo{
		JPEGPath: jpegPath,
		RAWPath:  findRAWPair(jpegPath),
		Name:     filepath.Base(jpegPath),
	}
}

func (p Photo) HasRAW() bool {
	return len(p.RAWPath) != 0
}

func IsRAWExtension(ext string) bool {
	for _, rawExt := range rawExtensions {
		if strings.EqualFold(ext, rawExt) {
			return true
		}
	}
	return false
}

func findRAWPair(jpegPath string) string {
	ext := filepath.Ext(jpegPath)
	base := strings.TrimSuffix(jpegPath, ext)
	for _, rawExt := range rawExtensions {
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
