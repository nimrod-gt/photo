//go:build !windows

package claudebin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBinariesAreTheBareName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"claude"}, binaries())
}

func TestDirsCoverTheUsualInstallLocations(t *testing.T) {
	t.Parallel()

	searched := dirs()
	assert.Contains(t, searched, "/usr/local/bin")
	assert.Contains(t, searched, "/opt/homebrew/bin")
}
