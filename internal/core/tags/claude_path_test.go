package tags

import (
	"errors"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupClaude(t *testing.T) {
	t.Parallel()

	notFound := func(string) (string, error) { return "", errors.New("not in PATH") }
	never := func(string) bool { return false }
	only := func(paths ...string) func(string) bool {
		return func(path string) bool {
			return slices.Contains(paths, path)
		}
	}

	t.Run("configured path wins", func(t *testing.T) {
		path, err := lookupClaude("/custom/claude", notFound, only("/custom/claude"), nil)
		require.NoError(t, err)
		assert.Equal(t, "/custom/claude", path)
	})

	t.Run("configured path is not executable", func(t *testing.T) {
		_, err := lookupClaude("/custom/claude", notFound, never, []string{"/usr/local/bin/claude"})
		require.ErrorIs(t, err, ErrClaudeNotFound)
		assert.Contains(t, err.Error(), "/custom/claude")
	})

	t.Run("falls back to PATH", func(t *testing.T) {
		lookPath := func(string) (string, error) { return "/usr/bin/claude", nil }
		path, err := lookupClaude("", lookPath, never, []string{"/usr/local/bin/claude"})
		require.NoError(t, err)
		assert.Equal(t, "/usr/bin/claude", path)
	})

	t.Run("probes candidates when PATH misses", func(t *testing.T) {
		candidates := []string{"/home/me/.local/bin/claude", "/usr/local/bin/claude"}
		path, err := lookupClaude("", notFound, only("/usr/local/bin/claude"), candidates)
		require.NoError(t, err)
		assert.Equal(t, "/usr/local/bin/claude", path)
	})

	t.Run("first matching candidate wins", func(t *testing.T) {
		candidates := []string{"/home/me/.local/bin/claude", "/usr/local/bin/claude"}
		path, err := lookupClaude("", notFound, only(candidates...), candidates)
		require.NoError(t, err)
		assert.Equal(t, "/home/me/.local/bin/claude", path)
	})

	t.Run("nothing found", func(t *testing.T) {
		_, err := lookupClaude("", notFound, never, []string{"/usr/local/bin/claude"})
		assert.ErrorIs(t, err, ErrClaudeNotFound)
	})
}

func TestClaudeCandidates(t *testing.T) {
	t.Parallel()

	candidates := claudeCandidates()
	require.NotEmpty(t, candidates)
	for _, candidate := range candidates {
		assert.Contains(t, filepath.Base(candidate), "claude")
		assert.True(t, filepath.IsAbs(candidate), candidate)
	}
	assert.Len(t, candidates, len(claudeDirs())*len(claudeBinaries()))
}

func TestClaudeBinaries(t *testing.T) {
	t.Parallel()

	binaries := claudeBinaries()
	require.NotEmpty(t, binaries)
	if runtime.GOOS == osWindows {
		assert.Contains(t, binaries, "claude.exe")
		assert.Contains(t, binaries, "claude.cmd")
		return
	}
	assert.Equal(t, []string{"claude"}, binaries)
}

func TestClaudeDirs(t *testing.T) {
	t.Parallel()

	dirs := claudeDirs()
	require.NotEmpty(t, dirs)
	if runtime.GOOS != osWindows {
		assert.Contains(t, dirs, "/usr/local/bin")
		assert.Contains(t, dirs, "/opt/homebrew/bin")
	}
	for _, dir := range dirs {
		assert.True(t, filepath.IsAbs(dir), dir)
	}
}

func TestIsExecutable(t *testing.T) {
	t.Parallel()

	assert.False(t, isExecutable(t.TempDir()))
	assert.False(t, isExecutable(t.TempDir()+"/missing"))
}
