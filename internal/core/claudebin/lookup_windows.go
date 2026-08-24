//go:build windows

package claudebin

import (
	"os"
	"path/filepath"
)

func binaries() []string {
	return []string{"claude.exe", "claude.cmd", "claude.bat"}
}

func dirs() []string {
	searched := homeDirs()
	if appData := os.Getenv("APPDATA"); len(appData) != 0 {
		searched = append(searched, filepath.Join(appData, "npm"))
	}
	if localAppData := os.Getenv("LOCALAPPDATA"); len(localAppData) != 0 {
		searched = append(searched, filepath.Join(localAppData, "Programs", "claude"))
	}
	return searched
}

// Windows has no execute bit; the extension is what makes a file runnable, and
// the candidates already carry one.
func executableMode(_ os.FileInfo) bool {
	return true
}
