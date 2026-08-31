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
			return byTime(a, b, a.ModTime, b.ModTime)
		})
	}
}

func SortPhotosByDates(photos []model.Photo, dates map[string]time.Time) {
	slices.SortFunc(photos, func(a, b model.Photo) int {
		return byTime(a, b, dates[a.ImagePath], dates[b.ImagePath])
	})
}

// A photo whose time was read sorts ahead of one whose was not, and those
// without fall back to the name. Taking the name mid-comparison instead puts a
// dated pair and an undated photo in a cycle, and a comparator that cycles
// leaves the order of the whole folder to the internals of the sort.
//
// Two photos of the same second fall back to the name as well: the sort is not
// a stable one, and a burst of frames all carrying the capture second would
// otherwise come back in a different order every time the folder is sorted.
func byTime(a, b model.Photo, ta, tb time.Time) int {
	switch {
	case !ta.IsZero() && !tb.IsZero():
		if c := ta.Compare(tb); c != 0 {
			return c
		}
	case !ta.IsZero():
		return -1
	case !tb.IsZero():
		return 1
	}
	return byName(a, b)
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
