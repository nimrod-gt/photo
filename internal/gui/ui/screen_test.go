package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaximizedContentSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		metrics screenMetrics
		want    fyne.Size
		wantOK  bool
	}{
		{
			name: "a panel along the top costs the same height off the bottom",
			metrics: screenMetrics{
				monitorHeight: 1080,
				areaTop:       40,
				area:          fyne.NewSize(1920, 1040),
			},
			want:   fyne.NewSize(1920, 1080-2*40-decorationAllowance),
			wantOK: true,
		},
		{
			name: "a dock along the bottom is not walked over either",
			metrics: screenMetrics{
				monitorHeight: 1080,
				areaTop:       0,
				area:          fyne.NewSize(1920, 1000),
			},
			want:   fyne.NewSize(1920, 2*1000-1080-decorationAllowance),
			wantOK: true,
		},
		{
			name: "a screen with nothing on it is taken whole",
			metrics: screenMetrics{
				monitorHeight: 1080,
				areaTop:       0,
				area:          fyne.NewSize(1920, 1080),
			},
			want:   fyne.NewSize(1920, 1080-decorationAllowance),
			wantOK: true,
		},
		{
			name: "a work area with no room left for a window",
			metrics: screenMetrics{
				monitorHeight: 1080,
				areaTop:       520,
				area:          fyne.NewSize(1920, 40),
			},
			wantOK: false,
		},
		{
			name: "no width reported",
			metrics: screenMetrics{
				monitorHeight: 1080,
				area:          fyne.NewSize(0, 1080),
			},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			size, ok := maximizedContentSize(tt.metrics)
			require.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.want, size)
			}
		})
	}
}

// The window is placed by centring it on the whole monitor, so a frame taller
// than this would put its title bar behind whatever sits along the top.
func TestCentrableHeightStaysInsideTheWorkArea(t *testing.T) {
	t.Parallel()

	metrics := screenMetrics{monitorHeight: 1080, areaTop: 40, area: fyne.NewSize(1920, 1040)}
	height := centrableHeight(metrics)

	top := (metrics.monitorHeight - height) / 2
	assert.GreaterOrEqual(t, top, metrics.areaTop)
	assert.LessOrEqual(t, top+height, metrics.areaTop+metrics.area.Height)
}
