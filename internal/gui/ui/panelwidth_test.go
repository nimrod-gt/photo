package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRescaledOffset(t *testing.T) {
	t.Parallel()

	const startingOffset = float64(leftPanelWidth) / defaultWindowWidth

	tests := []struct {
		name     string
		offset   float64
		oldWidth float32
		newWidth float32
		want     float64
	}{
		{
			name:     "maximizing keeps the panel at its width",
			offset:   startingOffset,
			oldWidth: defaultWindowWidth,
			newWidth: 3840,
			want:     float64(leftPanelWidth) / 3840,
		},
		{
			name:     "shrinking keeps the panel at its width",
			offset:   float64(leftPanelWidth) / 3840,
			oldWidth: 3840,
			newWidth: defaultWindowWidth,
			want:     startingOffset,
		},
		{
			name:     "a dragged divider is kept, not reset",
			offset:   0.5,
			oldWidth: defaultWindowWidth,
			newWidth: 2400,
			want:     0.25,
		},
		{
			name:     "a window narrower than the panel gives it everything",
			offset:   startingOffset,
			oldWidth: defaultWindowWidth,
			newWidth: 100,
			want:     1,
		},
		{
			name:     "no width to scale from",
			offset:   startingOffset,
			oldWidth: 0,
			newWidth: 3840,
			want:     startingOffset,
		},
		{
			name:     "no width to scale to",
			offset:   startingOffset,
			oldWidth: defaultWindowWidth,
			newWidth: 0,
			want:     startingOffset,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.InDelta(t, tt.want, rescaledOffset(tt.offset, tt.oldWidth, tt.newWidth), 1e-9)
		})
	}
}

func TestClampOffset(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0.25, clampOffset(0.25))
	assert.Equal(t, 0.0, clampOffset(-1))
	assert.Equal(t, 1.0, clampOffset(2))
}
