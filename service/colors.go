package service

import (
	"path/filepath"
	"slices"
	"sync"

	"photo/model"
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
	s.saveMu.Lock()
	defer s.saveMu.Unlock()

	dir := filepath.Dir(photo.ImagePath)

	s.mu.Lock()
	cm, err := s.loadOrGet(dir)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	cm.ToggleColor(photo.Name, color)
	snapshot := cm.Clone()
	s.mu.Unlock()

	return model.SaveColors(dir, snapshot)
}

func (s *ColorService) HasColor(photo model.Photo, color model.ColorLabel) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Dir(photo.ImagePath)
	cm, err := s.loadOrGet(dir)
	if err != nil {
		return false, err
	}
	return cm.HasColor(photo.Name, color), nil
}

func (s *ColorService) RemoveColors(photo model.Photo) error {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()

	dir := filepath.Dir(photo.ImagePath)

	s.mu.Lock()
	cm, err := s.loadOrGet(dir)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	if _, exists := cm[photo.Name]; !exists {
		s.mu.Unlock()
		return nil
	}
	delete(cm, photo.Name)
	snapshot := cm.Clone()
	s.mu.Unlock()

	return model.SaveColors(dir, snapshot)
}

func (s *ColorService) RemoveMultipleColors(photos []model.Photo) error {
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
		for _, name := range names {
			delete(cm, name)
		}
		snapshot := cm.Clone()
		s.mu.Unlock()

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

func (s *ColorService) InvalidateCache(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.colors, dir)
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
