//go:build windows

package claudebin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBinariesCarryAWindowsExtension(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"claude.exe", "claude.cmd", "claude.bat"}, binaries())
}
