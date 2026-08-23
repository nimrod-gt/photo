//go:build !windows

package proc

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHideLeavesTheCommandRunnable(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("/bin/sh", "-c", "exit 0")
	Hide(cmd)

	assert.Nil(t, cmd.SysProcAttr)
	require.NoError(t, cmd.Run())
}
