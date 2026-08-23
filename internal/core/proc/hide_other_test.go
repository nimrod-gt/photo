//go:build !windows

package proc

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHideLeavesTheCommandRunnable(t *testing.T) {
	cmd := exec.Command("true")
	Hide(cmd)

	assert.Nil(t, cmd.SysProcAttr)
	require.NoError(t, cmd.Run())
}
