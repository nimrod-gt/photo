package ui

import (
	"image/color"

	"photo/model"
)

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
