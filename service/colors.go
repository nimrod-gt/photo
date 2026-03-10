package service

import (
	"path/filepath"
	"sync"

	"photo/model"
)

type ColorService struct {
	mu     sync.Mutex
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
	return cm[photo.Name], nil
}

func (s *ColorService) ToggleColor(photo model.Photo, color model.ColorLabel) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Dir(photo.ImagePath)
	cm, err := s.loadOrGet(dir)
	if err != nil {
		return err
	}

	cm.ToggleColor(photo.Name, color)
	s.colors[dir] = cm

	return model.SaveColors(dir, cm)
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
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Dir(photo.ImagePath)
	cm, err := s.loadOrGet(dir)
	if err != nil {
		return err
	}

	if _, exists := cm[photo.Name]; !exists {
		return nil
	}

	delete(cm, photo.Name)
	s.colors[dir] = cm

	return model.SaveColors(dir, cm)
}

func (s *ColorService) RemoveMultipleColors(photos []model.Photo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	grouped := make(map[string][]string)
	for _, p := range photos {
		dir := filepath.Dir(p.ImagePath)
		grouped[dir] = append(grouped[dir], p.Name)
	}

	for dir, names := range grouped {
		cm, err := s.loadOrGet(dir)
		if err != nil {
			return err
		}
		for _, name := range names {
			delete(cm, name)
		}
		s.colors[dir] = cm
		if err := model.SaveColors(dir, cm); err != nil {
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

	result := make(model.ColorMap, len(cm))
	for k, v := range cm {
		copied := make([]model.ColorLabel, len(v))
		copy(copied, v)
		result[k] = copied
	}
	return result, nil
}

func (s *ColorService) InvalidateCache(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.colors, dir)
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
