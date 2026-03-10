package ui

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"

	"photo/model"
)

func TestEvictDistantThumbnails(t *testing.T) {
	makeMeta := func(n int) []model.PhotoMeta {
		meta := make([]model.PhotoMeta, n)
		for i := range meta {
			meta[i].Thumbnail = image.NewRGBA(image.Rect(0, 0, 1, 1))
		}
		return meta
	}

	hasThumbnail := func(meta []model.PhotoMeta, i int) bool {
		return meta[i].Thumbnail != nil
	}

	tests := []struct {
		name             string
		count            int
		lastVisibleIndex int
		wantLoIdx        int
		wantHiIdx        int
	}{
		{
			name:             "normal eviction in the middle",
			count:            200,
			lastVisibleIndex: 100,
			wantLoIdx:        50,
			wantHiIdx:        150,
		},
		{
			name:             "small set skipped",
			count:            80,
			lastVisibleIndex: 40,
			wantLoIdx:        0,
			wantHiIdx:        79,
		},
		{
			name:             "near beginning",
			count:            200,
			lastVisibleIndex: 10,
			wantLoIdx:        0,
			wantHiIdx:        60,
		},
		{
			name:             "near end",
			count:            200,
			lastVisibleIndex: 190,
			wantLoIdx:        140,
			wantHiIdx:        199,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gv := &GridViewer{
				meta:             makeMeta(tt.count),
				lastVisibleIndex: tt.lastVisibleIndex,
			}

			gv.evictDistantThumbnails()

			for i := range gv.meta {
				if i >= tt.wantLoIdx && i <= tt.wantHiIdx {
					assert.True(t, hasThumbnail(gv.meta, i), "index %d should have thumbnail", i)
				} else {
					assert.False(t, hasThumbnail(gv.meta, i), "index %d should be evicted", i)
				}
			}
		})
	}
}

func TestIsDistant(t *testing.T) {
	makeMeta := func(n int) []model.PhotoMeta {
		return make([]model.PhotoMeta, n)
	}

	tests := []struct {
		name             string
		count            int
		lastVisibleIndex int
		index            int
		want             bool
	}{
		{
			name:             "within range",
			count:            200,
			lastVisibleIndex: 100,
			index:            120,
			want:             false,
		},
		{
			name:             "at boundary low",
			count:            200,
			lastVisibleIndex: 100,
			index:            50,
			want:             false,
		},
		{
			name:             "at boundary high",
			count:            200,
			lastVisibleIndex: 100,
			index:            150,
			want:             false,
		},
		{
			name:             "beyond low",
			count:            200,
			lastVisibleIndex: 100,
			index:            49,
			want:             true,
		},
		{
			name:             "beyond high",
			count:            200,
			lastVisibleIndex: 100,
			index:            151,
			want:             true,
		},
		{
			name:             "small set never distant",
			count:            80,
			lastVisibleIndex: 40,
			index:            0,
			want:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gv := &GridViewer{
				meta:             makeMeta(tt.count),
				lastVisibleIndex: tt.lastVisibleIndex,
			}
			assert.Equal(t, tt.want, gv.isDistant(tt.index))
		})
	}
}
