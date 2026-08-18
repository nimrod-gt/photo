package tags

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const osWindows = "windows"

var ErrClaudeNotFound = errors.New("claude CLI not found")

// LookupClaude finds the claude binary. A desktop app started from Finder, the
// Dock or a Windows shortcut does not inherit the shell PATH, so exec.LookPath
// alone is not enough and the usual install locations are probed as well.
func LookupClaude(configured string) (string, error) {
	return lookupClaude(configured, exec.LookPath, isExecutable, claudeCandidates())
}

func lookupClaude(
	configured string,
	lookPath func(string) (string, error),
	executable func(string) bool,
	candidates []string,
) (string, error) {
	if len(configured) != 0 {
		if !executable(configured) {
			return "", fmt.Errorf("%s: %w", configured, ErrClaudeNotFound)
		}
		return configured, nil
	}

	// No extension on purpose: on Windows LookPath tries every PATHEXT suffix.
	if path, err := lookPath("claude"); err == nil {
		return path, nil
	}

	for _, candidate := range candidates {
		if executable(candidate) {
			return candidate, nil
		}
	}

	return "", ErrClaudeNotFound
}

func claudeBinaries() []string {
	if runtime.GOOS == osWindows {
		return []string{"claude.exe", "claude.cmd", "claude.bat"}
	}
	return []string{"claude"}
}

func claudeDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}

	var dirs []string
	if len(home) != 0 {
		dirs = append(dirs,
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, ".claude", "local"),
		)
	}

	if runtime.GOOS == osWindows {
		if appData := os.Getenv("APPDATA"); len(appData) != 0 {
			dirs = append(dirs, filepath.Join(appData, "npm"))
		}
		if localAppData := os.Getenv("LOCALAPPDATA"); len(localAppData) != 0 {
			dirs = append(dirs, filepath.Join(localAppData, "Programs", "claude"))
		}
		return dirs
	}

	return append(dirs, "/opt/homebrew/bin", "/usr/local/bin")
}

func claudeCandidates() []string {
	dirs := claudeDirs()
	binaries := claudeBinaries()

	candidates := make([]string, 0, len(dirs)*len(binaries))
	for _, dir := range dirs {
		for _, binary := range binaries {
			candidates = append(candidates, filepath.Join(dir, binary))
		}
	}
	return candidates
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == osWindows {
		return true
	}
	return info.Mode().Perm()&0o111 != 0
}
