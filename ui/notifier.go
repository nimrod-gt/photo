package ui

import (
	"image/color"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

const (
	notifierTimeout = 4 * time.Second
	notifierMargin  = float32(8)
	notifierBgAlpha = 200
)

type Notifier struct {
	mu        sync.Mutex
	timer     *time.Timer
	text      *canvas.Text
	bg        *canvas.Rectangle
	content   *fyne.Container
	container *fyne.Container
}

func NewNotifier() *Notifier {
	text := canvas.NewText("", color.White)
	text.TextSize = 13

	bg := canvas.NewRectangle(color.NRGBA{R: 200, G: 40, B: 40, A: notifierBgAlpha})
	bg.CornerRadius = 6

	content := container.NewStack(bg, container.NewPadded(text))
	content.Hide()

	n := &Notifier{
		text:      text,
		bg:        bg,
		content:   content,
		container: container.New(&bottomRightLayout{}, content),
	}
	return n
}

func (n *Notifier) Container() *fyne.Container {
	return n.container
}

func (n *Notifier) ShowError(msg string) {
	n.show(msg, color.NRGBA{R: 200, G: 40, B: 40, A: notifierBgAlpha})
}

func (n *Notifier) ShowWarning(msg string) {
	n.show(msg, color.NRGBA{R: 200, G: 170, B: 20, A: notifierBgAlpha})
}

func (n *Notifier) ShowNotification(msg string) {
	n.show(msg, color.NRGBA{R: 40, G: 140, B: 40, A: notifierBgAlpha})
}

func (n *Notifier) show(msg string, bgColor color.Color) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.text.Text = msg
	n.text.Refresh()
	n.bg.FillColor = bgColor
	n.bg.Refresh()
	n.content.Show()
	n.content.Refresh()

	if n.timer != nil {
		n.timer.Stop()
	}
	n.timer = time.AfterFunc(notifierTimeout, func() {
		fyne.Do(func() {
			n.content.Hide()
			n.content.Refresh()
		})
	})
}

type bottomRightLayout struct{}

func (l *bottomRightLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, 0)
}

func (l *bottomRightLayout) Layout(objects []fyne.CanvasObject, containerSize fyne.Size) {
	for _, obj := range objects {
		childSize := obj.MinSize()
		obj.Resize(childSize)
		obj.Move(fyne.NewPos(
			containerSize.Width-childSize.Width-notifierMargin,
			containerSize.Height-childSize.Height-notifierMargin,
		))
	}
}
