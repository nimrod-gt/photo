package model

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

type ColorLabel string

const (
	ColorRed   ColorLabel = "red"
	ColorGreen ColorLabel = "green"
	ColorBlue  ColorLabel = "blue"
)

const ColorsFileName = ".photo-colors.json"

type ColorMap map[string][]ColorLabel

func LoadColors(dir string) (ColorMap, error) {
	path := filepath.Join(dir, ColorsFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(ColorMap), nil
		}
		return nil, fmt.Errorf("reading colors file: %w", err)
	}
	var cm ColorMap
	if err := json.Unmarshal(data, &cm); err != nil {
		return nil, fmt.Errorf("parsing colors file: %w", err)
	}
	return cm, nil
}

func SaveColors(dir string, cm ColorMap) error {
	path := filepath.Join(dir, ColorsFileName)
	data, err := json.MarshalIndent(cm, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling colors: %w", err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("writing colors temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming colors temp file: %w", err)
	}
	return nil
}

func (cm ColorMap) Clone() ColorMap {
	result := make(ColorMap, len(cm))
	for k, v := range cm {
		result[k] = slices.Clone(v)
	}
	return result
}

func (cm ColorMap) HasColor(filename string, color ColorLabel) bool {
	return slices.Contains(cm[filename], color)
}

func (cm ColorMap) RemoveLabels(filename string, colors []ColorLabel) {
	remaining := slices.DeleteFunc(cm[filename], func(c ColorLabel) bool {
		return slices.Contains(colors, c)
	})
	if len(remaining) == 0 {
		delete(cm, filename)
		return
	}
	cm[filename] = remaining
}

func (cm ColorMap) ToggleColor(filename string, color ColorLabel) {
	colors := cm[filename]
	for i, c := range colors {
		if c == color {
			cm[filename] = append(colors[:i], colors[i+1:]...)
			if len(cm[filename]) == 0 {
				delete(cm, filename)
			}
			return
		}
	}
	cm[filename] = append(colors, color)
}
