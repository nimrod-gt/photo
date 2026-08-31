package library

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"photo/internal/core/model"
)

type SortOrder int

const (
	SortByName SortOrder = iota
	SortByTime
)

func ScanDirectory(dir string) ([]model.Photo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}

	names := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			names[entry.Name()] = true
		}
	}
	existsInDir := func(path string) bool {
		return names[filepath.Base(path)]
	}

	var photos []model.Photo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !model.IsSupportedImage(ext) {
			continue
		}
		fullPath := filepath.Join(dir, entry.Name())
		photo := model.NewPhotoWithExists(fullPath, existsInDir)
		if info, err := entry.Info(); err == nil {
			photo.ModTime = info.ModTime()
		}
		photos = append(photos, photo)
	}

	return photos, nil
}

func SortPhotos(photos []model.Photo, order SortOrder) {
	switch order {
	case SortByName:
		slices.SortFunc(photos, byName)
	case SortByTime:
		slices.SortFunc(photos, func(a, b model.Photo) int {
			if a.ModTime.IsZero() || b.ModTime.IsZero() {
				return byName(a, b)
			}
			return a.ModTime.Compare(b.ModTime)
		})
	}
}

func SortPhotosByDates(photos []model.Photo, dates map[string]time.Time) {
	slices.SortFunc(photos, func(a, b model.Photo) int {
		ta, oka := dates[a.ImagePath]
		tb, okb := dates[b.ImagePath]
		switch {
		case oka && okb:
			return ta.Compare(tb)
		case oka:
			// A photo whose date is known sorts ahead of one whose is not.
			return -1
		case okb:
			return 1
		default:
			return byName(a, b)
		}
	})
}

func byName(a, b model.Photo) int {
	return strings.Compare(a.Name, b.Name)
}

func ListDirectories(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", root, err)
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			dirs = append(dirs, filepath.Join(root, entry.Name()))
		}
	}
	return dirs, nil
}
