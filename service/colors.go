package service

import (
	"path/filepath"

	"photo/model"
)

type ColorService struct {
	colors map[string]model.ColorMap
}

func NewColorService() *ColorService {
	return &ColorService{
		colors: make(map[string]model.ColorMap),
	}
}

func (s *ColorService) GetColors(photo model.Photo) ([]model.ColorLabel, error) {
	dir := filepath.Dir(photo.JPEGPath)
	cm, err := s.loadOrGet(dir)
	if err != nil {
		return nil, err
	}
	return cm[photo.Name], nil
}

func (s *ColorService) ToggleColor(photo model.Photo, color model.ColorLabel) error {
	dir := filepath.Dir(photo.JPEGPath)
	cm, err := s.loadOrGet(dir)
	if err != nil {
		return err
	}

	cm.ToggleColor(photo.Name, color)
	s.colors[dir] = cm

	return model.SaveColors(dir, cm)
}

func (s *ColorService) HasColor(photo model.Photo, color model.ColorLabel) (bool, error) {
	dir := filepath.Dir(photo.JPEGPath)
	cm, err := s.loadOrGet(dir)
	if err != nil {
		return false, err
	}
	return cm.HasColor(photo.Name, color), nil
}

func (s *ColorService) RemoveColors(photo model.Photo) error {
	dir := filepath.Dir(photo.JPEGPath)
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

func (s *ColorService) InvalidateCache(dir string) {
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
