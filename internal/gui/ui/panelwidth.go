package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

// A split stores its divider as a ratio of the width, so every resize scales the
// file browser along with the window: maximizing a 1200px window onto a 3840px
// screen would blow the 250px panel up to 800px, and shrinking the window back
// would starve it. The panel keeps the width it has instead, whether that is the
// one it started with or the one the divider was last dragged to.
type panelWidthKeeper struct {
	split *container.Split
	width float32
}

func newPanelWidthKeeper(split *container.Split) *panelWidthKeeper {
	return &panelWidthKeeper{split: split, width: defaultWindowWidth}
}

func (k *panelWidthKeeper) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if size.Width > 0 && size.Width != k.width {
		k.split.SetOffset(rescaledOffset(k.split.Offset, k.width, size.Width))
		k.width = size.Width
	}
	for _, object := range objects {
		object.Move(fyne.NewPos(0, 0))
		object.Resize(size)
	}
}

func (k *panelWidthKeeper) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var size fyne.Size
	for _, object := range objects {
		size = size.Max(object.MinSize())
	}
	return size
}

func rescaledOffset(offset float64, oldWidth, newWidth float32) float64 {
	if oldWidth <= 0 || newWidth <= 0 {
		return offset
	}
	return clampOffset(offset * float64(oldWidth) / float64(newWidth))
}

func clampOffset(offset float64) float64 {
	return min(max(offset, 0), 1)
}
