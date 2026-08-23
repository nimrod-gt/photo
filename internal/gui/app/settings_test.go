package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"photo/internal/core/library"
)

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
