package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"photo/model"
)

type SortOrder int

const (
	SortByName SortOrder = iota
	SortByTime
)

type Scanner struct{}

func NewScanner() *Scanner {
	return &Scanner{}
}

func (s *Scanner) ScanDirectory(dir string) ([]model.Photo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
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
		photos = append(photos, model.NewPhoto(fullPath))
	}

	return photos, nil
}

func (s *Scanner) SortPhotos(photos []model.Photo, order SortOrder) {
	switch order {
	case SortByName:
		sort.Slice(photos, func(i, j int) bool {
			return photos[i].Name < photos[j].Name
		})
	case SortByTime:
		modTimes := make(map[string]time.Time, len(photos))
		for _, p := range photos {
			if info, err := os.Stat(p.ImagePath); err == nil {
				modTimes[p.ImagePath] = info.ModTime()
			}
		}
		sort.Slice(photos, func(i, j int) bool {
			ti, oki := modTimes[photos[i].ImagePath]
			tj, okj := modTimes[photos[j].ImagePath]
			if !oki || !okj {
				return photos[i].Name < photos[j].Name
			}
			return ti.Before(tj)
		})
	}
}

func (s *Scanner) SortPhotosByDates(photos []model.Photo, dates map[string]time.Time) {
	sort.Slice(photos, func(i, j int) bool {
		ti, oki := dates[photos[i].ImagePath]
		tj, okj := dates[photos[j].ImagePath]
		if !oki || !okj {
			return photos[i].Name < photos[j].Name
		}
		return ti.Before(tj)
	})
}

func (s *Scanner) ListDirectories(root string) ([]string, error) {
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
