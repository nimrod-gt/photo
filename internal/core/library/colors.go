package library

import (
	"path/filepath"
	"slices"
	"sync"

	"photo/internal/core/model"
)

// mu guards the in-memory cache only; saveMu serializes disk writes so a
// slow save never blocks readers, and snapshots reach disk in mutation order.
type ColorService struct {
	mu     sync.Mutex
	saveMu sync.Mutex
	colors map[string]model.ColorMap
}

func NewColorService() *ColorService {
	return &ColorService{
		colors: make(map[string]model.ColorMap),
	}
}

func (s *ColorService) GetColors(photo model.Photo) ([]model.ColorLabel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Dir(photo.ImagePath)
	cm, err := s.loadOrGet(dir)
	if err != nil {
		return nil, err
	}
	return slices.Clone(cm[photo.Name]), nil
}

func (s *ColorService) ToggleColor(photo model.Photo, color model.ColorLabel) error {
	return s.updateDirs([]model.Photo{photo}, func(cm model.ColorMap, name string) bool {
		cm.ToggleColor(name, color)
		return true
	})
}

func (s *ColorService) RemoveColors(photo model.Photo) error {
	return s.updateDirs([]model.Photo{photo}, func(cm model.ColorMap, name string) bool {
		if _, exists := cm[name]; !exists {
			return false
		}
		delete(cm, name)
		return true
	})
}

func (s *ColorService) RemoveMultipleColors(photos []model.Photo) error {
	return s.updateDirs(photos, func(cm model.ColorMap, name string) bool {
		delete(cm, name)
		return true
	})
}

func (s *ColorService) RemoveColorLabels(photos []model.Photo, colors []model.ColorLabel) error {
	return s.updateDirs(photos, func(cm model.ColorMap, name string) bool {
		cm.RemoveLabels(name, colors)
		return true
	})
}

func (s *ColorService) updateDirs(photos []model.Photo, apply func(model.ColorMap, string) bool) error {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()

	grouped := make(map[string][]string)
	for _, p := range photos {
		dir := filepath.Dir(p.ImagePath)
		grouped[dir] = append(grouped[dir], p.Name)
	}

	for dir, names := range grouped {
		s.mu.Lock()
		cm, err := s.loadOrGet(dir)
		if err != nil {
			s.mu.Unlock()
			return err
		}
		changed := false
		for _, name := range names {
			if apply(cm, name) {
				changed = true
			}
		}
		snapshot := cm.Clone()
		s.mu.Unlock()

		if !changed {
			continue
		}
		if err := model.SaveColors(dir, snapshot); err != nil {
			return err
		}
	}

	return nil
}

func (s *ColorService) GetDirectoryColors(dir string) (model.ColorMap, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cm, err := s.loadOrGet(dir)
	if err != nil {
		return nil, err
	}
	return cm.Clone(), nil
}

func (s *ColorService) ClearCache() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.colors = make(map[string]model.ColorMap)
}

func (s *ColorService) loadOrGet(dir string) (model.ColorMap, error) {
	if cm, ok := s.colors[dir]; ok {
		return cm, nil
	}

	cm, err := model.LoadColors(dir)
	if err != nil {
		return nil, err
	}

	s.colors[dir] = cm
	return cm, nil
}
