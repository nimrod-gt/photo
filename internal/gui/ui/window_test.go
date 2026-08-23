package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPanelOffset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		width float32
		want  float64
	}{
		{name: "default width", width: defaultWindowWidth, want: float64(leftPanelWidth) / defaultWindowWidth},
		{name: "maximized window keeps the panel at its width", width: 3840, want: float64(leftPanelWidth) / 3840},
		{name: "window narrower than the panel", width: 100, want: 1},
		{name: "no width reported", width: 0, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.InDelta(t, tt.want, panelOffset(tt.width), 1e-9)
		})
	}
}
