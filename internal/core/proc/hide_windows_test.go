//go:build windows

package proc

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHideKeepsTheConsoleAway(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "exit")
	Hide(cmd)

	require.NotNil(t, cmd.SysProcAttr)
	assert.True(t, cmd.SysProcAttr.HideWindow)
	assert.NotZero(t, cmd.SysProcAttr.CreationFlags&createNoWindow)
	require.NoError(t, cmd.Run())
}
