package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type DarkTheme struct{}

func NewDarkTheme() *DarkTheme {
	return &DarkTheme{}
}

func (t *DarkTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(name, theme.VariantDark)
}

func (t *DarkTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (t *DarkTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	switch name {
	case theme.IconNameMoveDown, theme.IconNameNavigateNext:
		return blankIcon
	}
	return theme.DefaultTheme().Icon(name)
}

var blankIcon = fyne.NewStaticResource("blank.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"/>`))

func (t *DarkTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameInlineIcon {
		return 0
	}
	return theme.DefaultTheme().Size(name)
}
