package ui

import (
	"image"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeThumbnailSource struct {
	peeked    []int
	sized     image.Image
	thumbnail image.Image
}

func (f *fakeThumbnailSource) Peek(_ string, size int) image.Image {
	f.peeked = append(f.peeked, size)
	return f.sized
}

func (f *fakeThumbnailSource) Thumbnail(string) image.Image {
	return f.thumbnail
}

func TestResolveThumbnail(t *testing.T) {
	t.Parallel()

	sized := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	cached := image.NewNRGBA(image.Rect(0, 0, 3, 3))
	embedded := image.NewNRGBA(image.Rect(0, 0, 2, 2))

	tests := []struct {
		name         string
		src          fakeThumbnailSource
		embedded     image.Image
		size         int
		want         image.Image
		wantNeedLoad bool
		wantPeeked   []int
	}{
		{
			name:       "sized hit wins",
			src:        fakeThumbnailSource{sized: sized, thumbnail: cached},
			embedded:   embedded,
			size:       256,
			want:       sized,
			wantPeeked: []int{256},
		},
		{
			name:         "sized miss falls back to the embedded thumbnail",
			src:          fakeThumbnailSource{thumbnail: cached},
			embedded:     embedded,
			size:         256,
			want:         embedded,
			wantNeedLoad: true,
			wantPeeked:   []int{256},
		},
		{
			name:         "sized miss without an embedded thumbnail",
			src:          fakeThumbnailSource{},
			size:         256,
			want:         nil,
			wantNeedLoad: true,
			wantPeeked:   []int{256},
		},
		{
			name:     "unsized prefers the cached thumbnail and never peeks",
			src:      fakeThumbnailSource{sized: sized, thumbnail: cached},
			embedded: embedded,
			size:     0,
			want:     cached,
		},
		{
			name:         "unsized falls back to the embedded thumbnail",
			src:          fakeThumbnailSource{sized: sized},
			embedded:     embedded,
			size:         0,
			want:         embedded,
			wantNeedLoad: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, needsLoad := resolveThumbnail(&tt.src, "DSC001.JPG", tt.embedded, tt.size)

			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantNeedLoad, needsLoad)
			assert.Equal(t, tt.wantPeeked, tt.src.peeked)
		})
	}
}

func TestUpdateThumbnailReuse(t *testing.T) {
	t.Parallel()

	thumb := newThumbnailImage(thumbnailWidth, thumbnailHeight)
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))

	updateThumbnail(thumb, img)
	require.Equal(t, img, thumb.Image)
	require.True(t, thumb.Visible())

	// the widget is reused for a photo with nothing cached yet
	updateThumbnail(thumb, nil)
	assert.Nil(t, thumb.Image)
	assert.False(t, thumb.Visible())
}

func TestNewThumbnailImage(t *testing.T) {
	t.Parallel()

	thumb := newThumbnailImage(thumbnailWidth, thumbnailHeight)

	assert.Equal(t, fyne.NewSize(thumbnailWidth, thumbnailHeight), thumb.MinSize())
	assert.Equal(t, canvas.ImageFillContain, thumb.FillMode)
}
