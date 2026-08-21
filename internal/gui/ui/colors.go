package ui

import (
	"image/color"

	"photo/internal/core/model"
)

const (
	favoriteMark = "★"
	colorMark    = "●"
)

var (
	colorOrder    = []model.ColorLabel{model.ColorRed, model.ColorGreen, model.ColorBlue}
	favoriteColor = color.NRGBA{R: 255, G: 215, B: 0, A: 255}
)

func ColorSet(colors []model.ColorLabel) map[model.ColorLabel]bool {
	set := make(map[model.ColorLabel]bool, len(colors))
	for _, c := range colors {
		set[c] = true
	}
	return set
}

func colorLabelToColor(label model.ColorLabel) color.Color {
	switch label {
	case model.ColorRed:
		return color.NRGBA{R: 255, G: 0, B: 0, A: 255}
	case model.ColorGreen:
		return color.NRGBA{R: 0, G: 255, B: 0, A: 255}
	case model.ColorBlue:
		return color.NRGBA{R: 0, G: 100, B: 255, A: 255}
	default:
		return color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	}
}
