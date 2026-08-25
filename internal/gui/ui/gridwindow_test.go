package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVisibleRangeBounds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		observed []int
		count    int
		buffer   int
		wantLo   int
		wantHi   int
	}{
		{
			name:   "nothing seen yet warms nothing",
			count:  1000,
			buffer: 20,
			wantLo: 0,
			wantHi: -1,
		},
		{
			name:     "a single tile warms the buffer around it",
			observed: []int{500},
			count:    1000,
			buffer:   20,
			wantLo:   480,
			wantHi:   520,
		},
		{
			name:     "a burst spans every tile seen since the last dispatch",
			observed: []int{505, 500, 511, 503},
			count:    1000,
			buffer:   20,
			wantLo:   480,
			wantHi:   531,
		},
		{
			name:     "the window clamps at the start",
			observed: []int{2},
			count:    1000,
			buffer:   20,
			wantLo:   0,
			wantHi:   22,
		},
		{
			name:     "the window clamps at the end",
			observed: []int{999},
			count:    1000,
			buffer:   20,
			wantLo:   979,
			wantHi:   999,
		},
		{
			name:     "an empty folder warms nothing",
			observed: []int{0},
			count:    0,
			buffer:   20,
			wantLo:   0,
			wantHi:   -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var r visibleRange
			for _, id := range tt.observed {
				r.observe(id)
			}

			lo, hi := r.bounds(tt.count, tt.buffer)

			assert.Equal(t, tt.wantLo, lo)
			assert.Equal(t, tt.wantHi, hi)
		})
	}
}

func TestVisibleRangeResetForgetsWhatWasSeen(t *testing.T) {
	t.Parallel()

	var r visibleRange
	r.observe(500)
	r.reset()

	lo, hi := r.bounds(1000, 20)
	assert.Equal(t, 0, lo)
	assert.Equal(t, -1, hi)

	// the range after a reset is the range of a fresh one, not one anchored at
	// the tile it last saw
	r.observe(700)
	lo, hi = r.bounds(1000, 20)
	assert.Equal(t, 680, lo)
	assert.Equal(t, 720, hi)
}
