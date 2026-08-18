//go:build windows

package clipboard

import (
	"fmt"
	"os/exec"
	"strings"
)

func CopyImage(path string) error {
	escaped := strings.ReplaceAll(path, `'`, `''`)
	script := fmt.Sprintf(`Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.Clipboard]::SetImage([System.Drawing.Image]::FromFile('%s'))`, escaped)
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("clipboard copy failed: %w (%s)", err, out)
	}
	return nil
}
