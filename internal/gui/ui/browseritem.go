package ui

import (
	"image"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"photo/internal/core/model"
)

const (
	thumbnailWidth  float32 = 48
	thumbnailHeight float32 = 36
	dateFontSize    float32 = 11
)

const (
	favoriteMark = "★"
	colorMark    = "●"
)

var favoriteColor = color.NRGBA{R: 255, G: 215, B: 0, A: 255}

type browserItem struct {
	widget.BaseWidget
	thumb    *canvas.Image
	name     *widget.Label
	star     *canvas.Text
	dots     *fyne.Container
	dateText *canvas.Text
	root     fyne.CanvasObject
}

func newBrowserItem() *browserItem {
	thumb := newThumbnailImage(thumbnailWidth, thumbnailHeight)

	nameLabel := widget.NewLabel("placeholder")
	nameLabel.Truncation = fyne.TextTruncateEllipsis

	star := canvas.NewText(favoriteMark, favoriteColor)
	star.TextSize = 14
	star.Hide()

	dotsContainer := newColorDots()

	topRow := container.NewBorder(nil, nil, nil, star, nameLabel)

	dateText := canvas.NewText("", color.NRGBA{R: 160, G: 160, B: 160, A: 255})
	dateText.TextSize = dateFontSize
	dateContent := container.New(layout.NewCustomPaddedLayout(0, 0, theme.InnerPadding(), 0), dateText)
	dateRow := container.NewBorder(nil, nil, nil, dotsContainer, dateContent)

	textBox := container.NewVBox(topRow, dateRow)

	item := &browserItem{
		thumb:    thumb,
		name:     nameLabel,
		star:     star,
		dots:     dotsContainer,
		dateText: dateText,
		root: container.NewBorder(nil, nil, thumb, nil,
			container.New(layout.NewStackLayout(), textBox),
		),
	}
	item.ExtendBaseWidget(item)
	return item
}

func (bi *browserItem) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(bi.root)
}

func (bi *browserItem) update(photo model.Photo, meta model.PhotoMeta, thumbnail image.Image) {
	updateThumbnail(bi.thumb, thumbnail)
	updateName(bi.name, photo.Name)
	updateStar(bi.star, meta.Favorite)
	updateColorDots(bi.dots, meta.Colors)
	updateDateText(bi.dateText, meta.Date)
}

func (fb *FileBrowser) createItem() fyne.CanvasObject {
	return newBrowserItem()
}

func (fb *FileBrowser) updateItem(id widget.ListItemID, obj fyne.CanvasObject) {
	photo, meta, ok := fb.data.itemAt(id)
	if !ok {
		return
	}

	item, ok := obj.(*browserItem)
	if !ok {
		return
	}

	thumbnail, _ := resolveThumbnail(fb.imageProvider, photo.ImagePath, meta.Thumbnail, 0)
	item.update(photo, meta, thumbnail)
}

func newColorDots() *fyne.Container {
	dots := make([]fyne.CanvasObject, len(colorOrder))
	for i := range dots {
		dot := canvas.NewText(colorMark, color.Transparent)
		dot.TextSize = 12
		dot.Hide()
		dots[i] = dot
	}
	return container.NewHBox(dots...)
}

func updateName(label *widget.Label, name string) {
	if label.Text == name {
		return
	}
	label.SetText(name)
}

func updateStar(star *canvas.Text, favorite bool) {
	if favorite {
		star.Show()
	} else {
		star.Hide()
	}
}

func updateColorDots(dotsContainer *fyne.Container, colors []model.ColorLabel) {
	for _, obj := range dotsContainer.Objects {
		obj.(*canvas.Text).Hide()
	}
	has := ColorSet(colors)
	idx := 0
	for _, c := range colorOrder {
		if !has[c] || idx >= len(dotsContainer.Objects) {
			continue
		}
		dot := dotsContainer.Objects[idx].(*canvas.Text)
		dot.Color = colorLabelToColor(c)
		dot.Show()
		dot.Refresh()
		idx++
	}
}

func updateDateText(dateText *canvas.Text, date time.Time) {
	if !date.IsZero() {
		dateText.Text = date.Format("2006-01-02 15:04")
	} else {
		dateText.Text = ""
	}
	dateText.Refresh()
}
