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

func rawExtensionVariants() []string {
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
}

func findRAWPair(jpegPath string) string {
	ext := filepath.Ext(jpegPath)
	base := strings.TrimSuffix(jpegPath, ext)
	for _, rawExt := range rawExtensionVariants() {
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
