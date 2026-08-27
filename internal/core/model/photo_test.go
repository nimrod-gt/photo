package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSupportedImage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ext      string
		expected bool
	}{
		{".jpg", true},
		{".JPG", true},
		{".jpeg", true},
		{".JPEG", true},
		{".png", true},
		{".PNG", true},
		{".arw", false},
		{".ARW", false},
		{".gif", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsSupportedImage(tt.ext))
		})
	}
}

func TestPhoto_IsJPEG(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     string
		expected bool
	}{
		{"/photos/a.jpg", true},
		{"/photos/a.JPG", true},
		{"/photos/a.jpeg", true},
		{"/photos/a.png", false},
		{"/photos/a.arw", false},
		{"/photos/noext", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.expected, Photo{ImagePath: tt.path}.IsJPEG())
		})
	}
}

func TestNewPhotoWithExists(t *testing.T) {
	t.Parallel()

	t.Run("finds ARW pair for JPEG", func(t *testing.T) {
		p := NewPhotoWithExists("/photos/img.jpg", func(path string) bool {
			return path == "/photos/img.ARW"
		})

		assert.Equal(t, "img.jpg", p.Name)
		assert.Equal(t, "/photos/img.ARW", p.RAWPath)
		assert.True(t, p.HasRAW())
	})

	t.Run("finds lowercase arw pair", func(t *testing.T) {
		p := NewPhotoWithExists("/photos/img.jpg", func(path string) bool {
			return path == "/photos/img.arw"
		})

		assert.Equal(t, "/photos/img.arw", p.RAWPath)
	})

	t.Run("no RAW pair", func(t *testing.T) {
		p := NewPhotoWithExists("/photos/img.jpg", func(string) bool { return false })

		assert.Empty(t, p.RAWPath)
		assert.False(t, p.HasRAW())
	})

	t.Run("PNG never gets RAW pair", func(t *testing.T) {
		p := NewPhotoWithExists("/photos/img.png", func(string) bool { return true })

		assert.Empty(t, p.RAWPath)
	})
}

func TestSidecarPath(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "/photos/DSC001.xmp", SidecarPath("/photos/DSC001.ARW"))
	assert.Equal(t, "/photos/DSC001.xmp", SidecarPath("/photos/DSC001.arw"))
	assert.Equal(t, "/photos/no-extension.xmp", SidecarPath("/photos/no-extension"))
}

func TestPhotoSidecarPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		photo Photo
		want  string
	}{
		{
			name:  "the RAW names the sidecar of a pair",
			photo: Photo{ImagePath: "/photos/DSC001.JPG", RAWPath: "/photos/DSC001.ARW"},
			want:  "/photos/DSC001.xmp",
		},
		{
			name:  "a JPEG without a pair names its own",
			photo: Photo{ImagePath: "/photos/DSC001.JPG"},
			want:  "/photos/DSC001.xmp",
		},
		{
			name:  "lowercase jpeg",
			photo: Photo{ImagePath: "/photos/DSC001.jpeg"},
			want:  "/photos/DSC001.xmp",
		},
		{
			name:  "a PNG names its own",
			photo: Photo{ImagePath: "/photos/shot.png"},
			want:  "/photos/shot.xmp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.photo.SidecarPath())
		})
	}
}
