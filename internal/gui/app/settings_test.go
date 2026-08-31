package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"photo/internal/core/library"
)

func TestSaveButtonVisible(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		show     bool
		autoXMP  bool
		autoJPEG bool
		want     bool
	}{
		{name: "hidden by the setting", show: false, want: false},
		{name: "nothing saved automatically", show: true, want: true},
		{name: "only the sidecar saved automatically", show: true, autoXMP: true, want: true},
		{name: "only the JPEG saved automatically", show: true, autoJPEG: true, want: true},
		{name: "both saved automatically", show: true, autoXMP: true, autoJPEG: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := &Application{showSaveButton: tt.show, autoSaveXMP: tt.autoXMP, autoSaveJPEG: tt.autoJPEG}
			assert.Equal(t, tt.want, a.saveButtonVisible())
		})
	}
}

func TestNormalizeSortOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value int
		want  library.SortOrder
	}{
		{name: "name", value: int(library.SortByName), want: library.SortByName},
		{name: "time", value: int(library.SortByTime), want: library.SortByTime},
		{name: "negative", value: -1, want: library.SortByName},
		{name: "above the last order", value: 7, want: library.SortByName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, normalizeSortOrder(tt.value))
		})
	}
}
