//go:build darwin

package service

import (
	"fmt"
	"os/exec"
	"strings"
)

func CopyImageToClipboard(path string) error {
	escaped := strings.ReplaceAll(path, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	script := fmt.Sprintf(`use framework "AppKit"
set img to current application's NSImage's alloc()'s initWithContentsOfFile:"%s"
if img is missing value then error "Failed to load image"
set pb to current application's NSPasteboard's generalPasteboard()
pb's clearContents()
pb's writeObjects:{img}`, escaped)
	cmd := exec.Command("osascript", "-l", "AppleScript", "-e", script) //nolint:gosec // path is from app-selected photo
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("clipboard copy failed: %w (%s)", err, out)
	}
	return nil
}
