package claudebin

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookup(t *testing.T) {
	t.Parallel()

	notFound := func(string) (string, error) { return "", errors.New("not in PATH") }
	never := func(string) bool { return false }
	only := func(paths ...string) func(string) bool {
		return func(path string) bool {
			return slices.Contains(paths, path)
		}
	}

	t.Run("configured path wins", func(t *testing.T) {
		path, err := lookup("/custom/claude", notFound, only("/custom/claude"), nil)
		require.NoError(t, err)
		assert.Equal(t, "/custom/claude", path)
	})

	t.Run("configured path is not executable", func(t *testing.T) {
		_, err := lookup("/custom/claude", notFound, never, []string{"/usr/local/bin/claude"})
		require.ErrorIs(t, err, ErrNotFound)
		assert.Contains(t, err.Error(), "/custom/claude")
	})

	t.Run("falls back to PATH", func(t *testing.T) {
		lookPath := func(string) (string, error) { return "/usr/bin/claude", nil }
		path, err := lookup("", lookPath, never, []string{"/usr/local/bin/claude"})
		require.NoError(t, err)
		assert.Equal(t, "/usr/bin/claude", path)
	})

	t.Run("probes candidates when PATH misses", func(t *testing.T) {
		paths := []string{"/home/me/.local/bin/claude", "/usr/local/bin/claude"}
		path, err := lookup("", notFound, only("/usr/local/bin/claude"), paths)
		require.NoError(t, err)
		assert.Equal(t, "/usr/local/bin/claude", path)
	})

	t.Run("first matching candidate wins", func(t *testing.T) {
		paths := []string{"/home/me/.local/bin/claude", "/usr/local/bin/claude"}
		path, err := lookup("", notFound, only(paths...), paths)
		require.NoError(t, err)
		assert.Equal(t, "/home/me/.local/bin/claude", path)
	})

	t.Run("nothing found", func(t *testing.T) {
		_, err := lookup("", notFound, never, []string{"/usr/local/bin/claude"})
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestCandidates(t *testing.T) {
	t.Parallel()

	paths := candidates()
	require.NotEmpty(t, paths)
	for _, path := range paths {
		assert.Contains(t, filepath.Base(path), "claude")
		assert.True(t, filepath.IsAbs(path), path)
	}
	assert.Len(t, paths, len(dirs())*len(binaries()))
}

func TestDirsAreAbsolute(t *testing.T) {
	t.Parallel()

	searched := dirs()
	require.NotEmpty(t, searched)
	for _, dir := range searched {
		assert.True(t, filepath.IsAbs(dir), dir)
	}
}

func TestIsExecutable(t *testing.T) {
	t.Parallel()

	assert.False(t, isExecutable(t.TempDir()))
	assert.False(t, isExecutable(t.TempDir()+"/missing"))
}
