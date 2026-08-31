package library

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"photo/internal/core/model"
)

func TestScanner_ScanDirectory(t *testing.T) {
	t.Parallel()

	t.Run("supported formats", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "a.jpg")
		touch(t, dir, "b.txt")
		touch(t, dir, "c.png")

		photos, err := ScanDirectory(dir)
		require.NoError(t, err)
		assert.Len(t, photos, 2)
		names := []string{photos[0].Name, photos[1].Name}
		assert.Contains(t, names, "a.jpg")
		assert.Contains(t, names, "c.png")
	})

	t.Run("case-insensitive extensions", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "a.JPG")
		touch(t, dir, "b.JPEG")
		touch(t, dir, "c.Jpg")

		photos, err := ScanDirectory(dir)
		require.NoError(t, err)
		assert.Len(t, photos, 3)
	})

	t.Run("skips directories", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "a.jpg")
		require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir.jpg"), 0755))

		photos, err := ScanDirectory(dir)
		require.NoError(t, err)
		assert.Len(t, photos, 1)
	})

	t.Run("empty directory", func(t *testing.T) {
		dir := t.TempDir()
		photos, err := ScanDirectory(dir)
		require.NoError(t, err)
		assert.Empty(t, photos)
	})

	t.Run("non-existent directory", func(t *testing.T) {
		_, err := ScanDirectory("/nonexistent/path")
		assert.Error(t, err)
	})

	t.Run("PNG has no RAW pair", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "photo.png")
		touch(t, dir, "photo.ARW")

		photos, err := ScanDirectory(dir)
		require.NoError(t, err)
		require.Len(t, photos, 1)
		assert.Equal(t, "photo.png", photos[0].Name)
		assert.False(t, photos[0].HasRAW())
	})

	t.Run("with RAW pair", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "photo.jpg")
		touch(t, dir, "photo.ARW")

		photos, err := ScanDirectory(dir)
		require.NoError(t, err)
		require.Len(t, photos, 1)
		assert.Equal(t, "photo.jpg", photos[0].Name)
		assert.True(t, photos[0].HasRAW())
	})

	t.Run("with lowercase RAW pair", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "photo.jpg")
		touch(t, dir, "photo.arw")

		photos, err := ScanDirectory(dir)
		require.NoError(t, err)
		require.Len(t, photos, 1)
		assert.True(t, photos[0].HasRAW())
	})

	t.Run("populates ModTime", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "a.jpg")

		photos, err := ScanDirectory(dir)
		require.NoError(t, err)
		require.Len(t, photos, 1)
		assert.False(t, photos[0].ModTime.IsZero())
		assert.WithinDuration(t, time.Now(), photos[0].ModTime, time.Minute)
	})
}

func TestScanner_SortPhotos(t *testing.T) {
	t.Parallel()

	t.Run("by name", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "c.jpg")
		touch(t, dir, "a.jpg")
		touch(t, dir, "b.jpg")

		photos, err := ScanDirectory(dir)
		require.NoError(t, err)

		SortPhotos(photos, SortByName)
		assert.Equal(t, "a.jpg", photos[0].Name)
		assert.Equal(t, "b.jpg", photos[1].Name)
		assert.Equal(t, "c.jpg", photos[2].Name)
	})

	t.Run("by time", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "old.jpg")
		touch(t, dir, "new.jpg")

		now := time.Now()
		require.NoError(t, os.Chtimes(filepath.Join(dir, "old.jpg"), now.Add(-2*time.Hour), now.Add(-2*time.Hour)))
		require.NoError(t, os.Chtimes(filepath.Join(dir, "new.jpg"), now, now))

		photos, err := ScanDirectory(dir)
		require.NoError(t, err)

		SortPhotos(photos, SortByTime)
		assert.Equal(t, "old.jpg", photos[0].Name)
		assert.Equal(t, "new.jpg", photos[1].Name)
	})

	t.Run("by time uses ModTime without touching disk", func(t *testing.T) {
		now := time.Now()
		photos := []model.Photo{
			{ImagePath: "/nonexistent/b.jpg", Name: "b.jpg", ModTime: now},
			{ImagePath: "/nonexistent/a.jpg", Name: "a.jpg", ModTime: now.Add(-time.Hour)},
		}

		SortPhotos(photos, SortByTime)
		assert.Equal(t, "a.jpg", photos[0].Name)
		assert.Equal(t, "b.jpg", photos[1].Name)
	})

	t.Run("by time puts a photo with no ModTime after the dated ones", func(t *testing.T) {
		photos := []model.Photo{
			{ImagePath: "/nonexistent/b.jpg", Name: "b.jpg", ModTime: time.Now()},
			{ImagePath: "/nonexistent/a.jpg", Name: "a.jpg"},
		}

		SortPhotos(photos, SortByTime)
		assert.Equal(t, []string{"b.jpg", "a.jpg"}, photoNames(photos))
	})

	// Taking the name whenever either side had no ModTime made these three
	// cycle: a < m and m < z by name, while z < a by time.
	t.Run("by time orders a dated pair against an undated photo", func(t *testing.T) {
		photos := []model.Photo{
			{ImagePath: "/nonexistent/m.jpg", Name: "m.jpg"},
			{ImagePath: "/nonexistent/a.jpg", Name: "a.jpg", ModTime: time.Date(2026, time.August, 9, 0, 0, 0, 0, time.UTC)},
			{ImagePath: "/nonexistent/z.jpg", Name: "z.jpg", ModTime: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)},
		}

		SortPhotos(photos, SortByTime)
		assert.Equal(t, []string{"z.jpg", "a.jpg", "m.jpg"}, photoNames(photos))
	})

	// A burst of frames carries one capture second, and the sort is not stable:
	// without the name the order of the burst is whatever the sort leaves.
	t.Run("by time falls back to the name on the same moment", func(t *testing.T) {
		shot := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)
		for _, order := range [][]string{{"c.jpg", "a.jpg", "b.jpg"}, {"b.jpg", "c.jpg", "a.jpg"}} {
			photos := make([]model.Photo, 0, len(order))
			for _, name := range order {
				photos = append(photos, model.Photo{ImagePath: "/nonexistent/" + name, Name: name, ModTime: shot})
			}

			SortPhotos(photos, SortByTime)
			assert.Equal(t, []string{"a.jpg", "b.jpg", "c.jpg"}, photoNames(photos))
		}
	})
}

// The dates come from the browser's meta rather than from the files, so every
// case here is built in memory: what is under test is the order the comparator
// puts the photos in, not what the disk says about them.
func TestScanner_SortPhotosByDates(t *testing.T) {
	t.Parallel()

	photo := func(name string) model.Photo {
		return model.Photo{ImagePath: "/nonexistent/" + name, Name: name}
	}
	day := func(n int) time.Time {
		return time.Date(2026, time.August, n, 0, 0, 0, 0, time.UTC)
	}

	tests := []struct {
		name   string
		photos []model.Photo
		dates  map[string]time.Time
		want   []string
	}{
		{
			name:   "known dates sort chronologically, not by name",
			photos: []model.Photo{photo("a.jpg"), photo("b.jpg"), photo("c.jpg")},
			dates: map[string]time.Time{
				"/nonexistent/a.jpg": day(3),
				"/nonexistent/b.jpg": day(1),
				"/nonexistent/c.jpg": day(2),
			},
			want: []string{"b.jpg", "c.jpg", "a.jpg"},
		},
		{
			// The name order is against the date order in both rows, so a pass
			// cannot come from the input happening to be sorted already.
			name:   "a known date sorts ahead of an unknown one",
			photos: []model.Photo{photo("a.jpg"), photo("z.jpg")},
			dates:  map[string]time.Time{"/nonexistent/z.jpg": day(1)},
			want:   []string{"z.jpg", "a.jpg"},
		},
		{
			name:   "the same, with the arguments the other way round",
			photos: []model.Photo{photo("z.jpg"), photo("a.jpg")},
			dates:  map[string]time.Time{"/nonexistent/a.jpg": day(1)},
			want:   []string{"a.jpg", "z.jpg"},
		},
		{
			// Where the ties actually come from: a capture date is read to the
			// second, and a burst of frames shares one.
			name:   "photos of the same moment fall back to the name",
			photos: []model.Photo{photo("c.jpg"), photo("a.jpg"), photo("b.jpg")},
			dates: map[string]time.Time{
				"/nonexistent/a.jpg": day(1),
				"/nonexistent/b.jpg": day(1),
				"/nonexistent/c.jpg": day(1),
			},
			want: []string{"a.jpg", "b.jpg", "c.jpg"},
		},
		{
			name:   "photos without dates fall back to the name",
			photos: []model.Photo{photo("c.jpg"), photo("a.jpg"), photo("b.jpg")},
			dates:  nil,
			want:   []string{"a.jpg", "b.jpg", "c.jpg"},
		},
		{
			name:   "the dated ones lead, the rest follow by name",
			photos: []model.Photo{photo("a.jpg"), photo("b.jpg"), photo("y.jpg"), photo("z.jpg")},
			dates: map[string]time.Time{
				"/nonexistent/z.jpg": day(1),
				"/nonexistent/y.jpg": day(2),
			},
			want: []string{"z.jpg", "y.jpg", "a.jpg", "b.jpg"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			SortPhotosByDates(tt.photos, tt.dates)
			assert.Equal(t, tt.want, photoNames(tt.photos))
		})
	}
}

func TestScanner_ListDirectories(t *testing.T) {
	t.Parallel()

	t.Run("basic", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, "subA"), 0755))
		require.NoError(t, os.Mkdir(filepath.Join(dir, "subB"), 0755))
		touch(t, dir, "file.txt")

		dirs, err := ListDirectories(dir)
		require.NoError(t, err)
		assert.Len(t, dirs, 2)
	})

	t.Run("excludes hidden", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, ".hidden"), 0755))
		require.NoError(t, os.Mkdir(filepath.Join(dir, "visible"), 0755))

		dirs, err := ListDirectories(dir)
		require.NoError(t, err)
		assert.Len(t, dirs, 1)
		assert.Contains(t, dirs[0], "visible")
	})
}

func photoNames(photos []model.Photo) []string {
	names := make([]string, 0, len(photos))
	for _, p := range photos {
		names = append(names, p.Name)
	}
	return names
}

func touch(t *testing.T, dir, name string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), nil, 0600))
}
