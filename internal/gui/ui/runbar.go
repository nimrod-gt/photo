package ui

import (
	"fmt"
	"image/color"
	"slices"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	runBarWidthPct = float32(0.5)
	runBarTick     = time.Second
	// The label is given exactly the width its text measures at, and a plate cut
	// to that lands on the very edge where truncation starts.
	runBarTextSlack = float32(4)
)

type RunItem struct {
	Name  string
	Since time.Time
}

// The time comes before the name, because the plate is only as wide as its share
// of the photo and what runs past that is cut: a name loses its tail and stays
// recognisable, while a clock that lost its digits says nothing at all.
type runRow struct {
	label   *widget.Label
	content *fyne.Container
}

func newRunRow() *runRow {
	label := widget.NewLabel("")
	label.Truncation = fyne.TextTruncateEllipsis

	bg := canvas.NewRectangle(color.NRGBA{R: 20, G: 20, B: 20, A: tagBarBgAlpha})
	bg.CornerRadius = 6

	// A Label reserves theme.InnerPadding around its text and cannot be told
	// otherwise, so the plate is pulled back in by that much on the sides the
	// label touches and given the padding it should have - the same trick the
	// tag bar plays, with the icon taking the left edge for itself.
	inset := theme.InnerPadding()
	// Border rather than a box: a box hands each child the width it asks for, and
	// a truncating label asks for the width of the ellipsis alone - which is all
	// it would then ever show. The border stretches the label over what is left
	// of the plate instead.
	row := container.NewBorder(nil, nil, container.NewCenter(widget.NewIcon(theme.ViewRefreshIcon())), nil, label)
	padded := container.New(
		layout.NewCustomPaddedLayout(tagBarTextPadY-inset, tagBarTextPadY-inset, tagBarTextPadX, tagBarTextPadX-inset),
		row,
	)

	return &runRow{label: label, content: container.NewStack(bg, padded)}
}

// The width the row asks for, which a truncating label does not report: its
// MinSize is the width of the ellipsis alone, so a plate sized from it would be
// a stub showing nothing else.
func (r *runRow) width() float32 {
	text := fyne.MeasureText(r.label.Text, theme.TextSize(), r.label.TextStyle).Width
	// The label keeps its own padding on the left, where the icon is, and the
	// border puts its own between the two; both are part of the plate.
	return theme.IconInlineSize() + theme.Padding() + theme.InnerPadding() + text + 2*tagBarTextPadX + runBarTextSlack
}

// The plates stack in the top-right corner of the photo, one per generation
// still going, leaving the other three corners to the tag bar and the notifier.
// The bar is told the whole list at once rather than about single runs starting
// and ending: the runner keeps that list anyway, and a corner that mirrors it is
// never left with a plate for a run nobody is waiting on any more.
//
// The rows and the list they are built from are locked rather than left to the
// UI goroutine: a run reports itself through fyne.Do, which hops to that
// goroutine in the app but runs where it stands under the test driver, and a
// rule only one of the two obeys is no rule at all.
type RunBar struct {
	mu        sync.Mutex
	timer     *time.Timer
	items     []RunItem
	layout    *topRightLayout
	container *fyne.Container
}

func NewRunBar() *RunBar {
	bar := &RunBar{layout: &topRightLayout{}}
	bar.container = container.New(bar.layout)
	bar.container.Hide()
	return bar
}

func (b *RunBar) Container() *fyne.Container {
	return b.container
}

func (b *RunBar) SetRuns(items []RunItem) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.items = slices.Clone(items)

	rows := make([]*runRow, 0, len(b.items))
	objects := make([]fyne.CanvasObject, 0, len(b.items))
	for range b.items {
		row := newRunRow()
		rows = append(rows, row)
		objects = append(objects, row.content)
	}
	b.layout.rows = rows
	b.container.Objects = objects
	showIf(b.container, len(b.items) != 0)

	b.relabel()
	b.schedule()
}

// The rows are relabelled rather than rebuilt, so the running time moves without
// the plates being replaced under the canvas every second.
func (b *RunBar) relabel() {
	for i, item := range b.items {
		b.layout.rows[i].label.SetText(runElapsed(item.Since) + "  " + item.Name)
	}
	// The container, not the rows: the widths come from topRightLayout, and a
	// truncating label reports the same MinSize whatever its text, so nothing
	// else would re-run the layout - the same reason TagBar.refresh does it.
	b.container.Refresh()
}

// A tick is only armed while something is running, so an idle app is left alone
// instead of being woken every second for a corner with nothing in it.
func (b *RunBar) schedule() {
	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}
	if len(b.items) == 0 {
		return
	}
	b.timer = time.AfterFunc(runBarTick, b.tick)
}

func (b *RunBar) tick() {
	fyne.Do(func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		b.relabel()
		b.schedule()
	})
}

// A run with no start time behind it - nothing in the app makes one, but a zero
// time is what a struct left half filled carries - reads as having just begun
// rather than as two thousand years old.
func runElapsed(since time.Time) string {
	if since.IsZero() {
		return "0:00"
	}
	seconds := max(int(time.Since(since)/time.Second), 0)
	if seconds >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", seconds/3600, seconds/60%60, seconds%60)
	}
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

type topRightLayout struct {
	rows []*runRow
}

func (l *topRightLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, 0)
}

func (l *topRightLayout) Layout(_ []fyne.CanvasObject, containerSize fyne.Size) {
	top := tagBarMargin
	for _, row := range l.rows {
		limit := max(min(containerSize.Width*runBarWidthPct, containerSize.Width-2*tagBarMargin), 0)
		size := fyne.NewSize(min(row.width(), limit), row.content.MinSize().Height)
		row.content.Resize(size)
		row.content.Move(fyne.NewPos(max(containerSize.Width-size.Width-tagBarMargin, 0), top))
		top += size.Height + tagBarRowGap
	}
}
