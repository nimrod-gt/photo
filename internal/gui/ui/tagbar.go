package ui

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"photo/internal/core/model"
)

const (
	tagBarMargin      = float32(8)
	tagBarBgAlpha     = 190
	tagBarMaxWidthPct = 0.6
)

type TagBar struct {
	title      *widget.Label
	keywords   *widget.Label
	content    *fyne.Container
	container  *fyne.Container
	tags       model.Tags
	enabled    bool
	suppressed bool
}

func NewTagBar() *TagBar {
	title := widget.NewLabel("")
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Truncation = fyne.TextTruncateEllipsis

	keywords := widget.NewLabel("")
	keywords.Truncation = fyne.TextTruncateEllipsis

	bg := canvas.NewRectangle(color.NRGBA{R: 20, G: 20, B: 20, A: tagBarBgAlpha})
	bg.CornerRadius = 6

	content := container.NewStack(bg, container.NewPadded(container.NewVBox(title, keywords)))
	content.Hide()

	bar := &TagBar{
		title:    title,
		keywords: keywords,
		content:  content,
		enabled:  true,
	}
	bar.container = container.New(&bottomLeftLayout{bar: bar}, content)
	return bar
}

func (b *TagBar) Container() *fyne.Container {
	return b.container
}

func (b *TagBar) SetTags(tags model.Tags) {
	b.tags = tags
	b.title.SetText(tags.Title)
	b.keywords.SetText(tags.KeywordLine())
	b.refresh()
}

func (b *TagBar) Clear() {
	b.SetTags(model.Tags{})
}

func (b *TagBar) SetEnabled(enabled bool) {
	b.enabled = enabled
	b.refresh()
}

func (b *TagBar) SetSuppressed(suppressed bool) {
	b.suppressed = suppressed
	b.refresh()
}

// The overlay describes the photo underneath, so it is there only while it has
// something to say: a photo without tags keeps the whole image uncovered.
func (b *TagBar) refresh() {
	if !b.enabled || b.suppressed || b.tags.IsEmpty() {
		b.content.Hide()
		return
	}
	showIf(b.title, len(strings.TrimSpace(b.tags.Title)) != 0)
	showIf(b.keywords, len(b.tags.Keywords) != 0)
	b.content.Show()
	b.content.Refresh()
}

func showIf(obj fyne.CanvasObject, visible bool) {
	if visible {
		obj.Show()
		return
	}
	obj.Hide()
}

// The width the text asks for, which a truncating label does not report: its
// MinSize is the width of the ellipsis alone, so an overlay sized from it would
// be a stub showing nothing else.
func (b *TagBar) textWidth() float32 {
	var widest float32
	for _, label := range []*widget.Label{b.title, b.keywords} {
		if !label.Visible() {
			continue
		}
		widest = max(widest, fyne.MeasureText(label.Text, theme.TextSize(), label.TextStyle).Width)
	}
	return widest + 2*theme.InnerPadding() + 2*theme.Padding()
}

// The overlay sits in the bottom-left corner, leaving the bottom-right one to
// the notifier, and is capped at a share of the photo so a long title truncates
// instead of stretching across the whole image.
type bottomLeftLayout struct {
	bar *TagBar
}

func (l *bottomLeftLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, 0)
}

func (l *bottomLeftLayout) Layout(objects []fyne.CanvasObject, containerSize fyne.Size) {
	maxWidth := max(min(containerSize.Width*tagBarMaxWidthPct, containerSize.Width-2*tagBarMargin), 0)
	width := min(l.bar.textWidth(), maxWidth)
	for _, obj := range objects {
		size := fyne.NewSize(width, obj.MinSize().Height)
		obj.Resize(size)
		obj.Move(fyne.NewPos(tagBarMargin, max(containerSize.Height-size.Height-tagBarMargin, 0)))
	}
}
