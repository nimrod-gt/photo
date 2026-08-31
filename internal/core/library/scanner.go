package library

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
		sort.Slice(photos, func(i, j int) bool {
			return photos[i].Name < photos[j].Name
		})
	case SortByTime:
		sort.Slice(photos, func(i, j int) bool {
			ti, tj := photos[i].ModTime, photos[j].ModTime
			if ti.IsZero() || tj.IsZero() {
				return photos[i].Name < photos[j].Name
			}
			return ti.Before(tj)
		})
	}
}

func SortPhotosByDates(photos []model.Photo, dates map[string]time.Time) {
	sort.Slice(photos, func(i, j int) bool {
		ti, oki := dates[photos[i].ImagePath]
		tj, okj := dates[photos[j].ImagePath]
		if oki && okj {
			return ti.Before(tj)
		}
		if oki != okj {
			return oki
		}
		return photos[i].Name < photos[j].Name
	})
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
