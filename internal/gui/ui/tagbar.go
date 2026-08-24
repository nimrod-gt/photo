package ui

import (
	"image/color"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"photo/internal/core/model"
)

const (
	tagBarMargin        = float32(8)
	tagBarRowGap        = float32(4)
	tagBarBgAlpha       = 190
	tagBarMaxWidthPct   = 0.6
	tagBarTitleWidthPct = 0.275
	tagBarTextPadX      = float32(6)
	tagBarTextPadY      = float32(2)
)

type tagRow struct {
	label    *widget.Label
	content  *fyne.Container
	widthPct float32
}

func newTagRow(label *widget.Label, widthPct float32) *tagRow {
	label.Truncation = fyne.TextTruncateEllipsis

	bg := canvas.NewRectangle(color.NRGBA{R: 20, G: 20, B: 20, A: tagBarBgAlpha})
	bg.CornerRadius = 6

	// A Label reserves theme.InnerPadding around its text and cannot be told
	// otherwise, so the plate is pulled back in by that much and given the
	// padding it should have. Negative values are what CustomPaddedLayout takes
	// for it.
	inset := theme.InnerPadding()
	text := container.New(
		layout.NewCustomPaddedLayout(tagBarTextPadY-inset, tagBarTextPadY-inset, tagBarTextPadX-inset, tagBarTextPadX-inset),
		label,
	)
	content := container.NewStack(bg, text)
	content.Hide()

	return &tagRow{label: label, content: content, widthPct: widthPct}
}

// The width the text asks for, which a truncating label does not report: its
// MinSize is the width of the ellipsis alone, so a row sized from it would be a
// stub showing nothing else.
func (r *tagRow) textWidth() float32 {
	return fyne.MeasureText(r.label.Text, theme.TextSize(), r.label.TextStyle).Width + 2*tagBarTextPadX
}

type TagBar struct {
	title      *tagRow
	keywords   *tagRow
	container  *fyne.Container
	tags       model.Tags
	enabled    bool
	suppressed bool
}

func NewTagBar() *TagBar {
	title := widget.NewLabel("")
	title.TextStyle = fyne.TextStyle{Bold: true}

	bar := &TagBar{
		title:    newTagRow(title, tagBarTitleWidthPct),
		keywords: newTagRow(widget.NewLabel(""), tagBarMaxWidthPct),
		enabled:  true,
	}
	rows := []*tagRow{bar.title, bar.keywords}
	bar.container = container.New(&bottomLeftLayout{rows: rows}, bar.title.content, bar.keywords.content)
	return bar
}

func (b *TagBar) Container() *fyne.Container {
	return b.container
}

func (b *TagBar) SetTags(tags model.Tags) {
	b.tags = tags
	b.title.label.SetText(tags.Title)
	b.keywords.label.SetText(tags.KeywordLine())
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
// something to say: a photo without tags keeps the whole image uncovered, and a
// photo with only one of the two keeps the other row away.
func (b *TagBar) refresh() {
	shown := b.enabled && !b.suppressed && !b.tags.IsEmpty()
	showIf(b.title.content, shown && len(strings.TrimSpace(b.tags.Title)) != 0)
	showIf(b.keywords.content, shown && len(b.tags.Keywords) != 0)
	// The outer container, not the rows: the widths come from bottomLeftLayout,
	// and refreshing a row alone re-runs only its own layout. Nothing else would
	// re-run the outer one either, because a truncating label reports the same
	// MinSize whatever its text, so the canvas sees no change to react to.
	b.container.Refresh()
}

func showIf(obj fyne.CanvasObject, visible bool) {
	if visible {
		obj.Show()
		return
	}
	obj.Hide()
}

// The rows stack in the bottom-left corner, leaving the bottom-right one to the
// notifier. Each is its own plate, as wide as its own text and no wider than its
// share of the photo, so a long title truncates instead of stretching across the
// image and the keywords are not dragged out to the width of the title.
type bottomLeftLayout struct {
	rows []*tagRow
}

func (l *bottomLeftLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, 0)
}

func (l *bottomLeftLayout) Layout(_ []fyne.CanvasObject, containerSize fyne.Size) {
	bottom := max(containerSize.Height-tagBarMargin, 0)
	for _, row := range slices.Backward(l.rows) {

		if !row.content.Visible() {
			continue
		}
		limit := max(min(containerSize.Width*row.widthPct, containerSize.Width-2*tagBarMargin), 0)
		size := fyne.NewSize(min(row.textWidth(), limit), row.content.MinSize().Height)
		row.content.Resize(size)
		row.content.Move(fyne.NewPos(tagBarMargin, max(bottom-size.Height, 0)))
		bottom = max(bottom-size.Height-tagBarRowGap, 0)
	}
}
