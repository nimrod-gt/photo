package model

import (
	"path/filepath"
	"strings"
)

var rawExtensions = map[string]bool{
	".arw": true,
	".ARW": true,
}

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
	return rawExtensions[ext]
}

func findRAWPair(jpegPath string) string {
	ext := filepath.Ext(jpegPath)
	base := strings.TrimSuffix(jpegPath, ext)
	for rawExt := range rawExtensions {
		candidate := base + rawExt
		if candidate != jpegPath {
			return candidate
		}
	}
	return ""
}
