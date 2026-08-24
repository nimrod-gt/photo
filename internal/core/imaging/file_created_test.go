package imaging

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileCreated(t *testing.T) {
	t.Parallel()

	t.Run("a file just written is dated now", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "photo.jpg")
		before := time.Now().Add(-time.Minute)
		require.NoError(t, os.WriteFile(path, []byte("data"), 0o600))

		created := fileCreated(path)

		assert.False(t, created.IsZero())
		assert.True(t, created.After(before), "created %v is not after %v", created, before)
		assert.False(t, created.After(time.Now().Add(time.Minute)))
	})

	t.Run("a missing file has no date", func(t *testing.T) {
		assert.True(t, fileCreated(filepath.Join(t.TempDir(), "gone.jpg")).IsZero())
	})
}
