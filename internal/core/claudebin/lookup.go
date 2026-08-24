package claudebin

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

var ErrNotFound = errors.New("claude CLI not found")

// Lookup finds the claude binary. A desktop app started from Finder, the Dock
// or a Windows shortcut does not inherit the shell PATH, so exec.LookPath alone
// is not enough and the usual install locations are probed as well.
func Lookup(configured string) (string, error) {
	return lookup(configured, exec.LookPath, isExecutable, candidates())
}

func lookup(
	configured string,
	lookPath func(string) (string, error),
	executable func(string) bool,
	paths []string,
) (string, error) {
	if len(configured) != 0 {
		if !executable(configured) {
			return "", fmt.Errorf("%s: %w", configured, ErrNotFound)
		}
		return configured, nil
	}

	// No extension on purpose: on Windows LookPath tries every PATHEXT suffix.
	if path, err := lookPath("claude"); err == nil {
		return path, nil
	}

	for _, path := range paths {
		if executable(path) {
			return path, nil
		}
	}

	return "", ErrNotFound
}

func candidates() []string {
	searched := dirs()
	names := binaries()

	paths := make([]string, 0, len(searched)*len(names))
	for _, dir := range searched {
		for _, binary := range names {
			paths = append(paths, filepath.Join(dir, binary))
		}
	}
	return paths
}

func homeDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil || len(home) == 0 {
		return nil
	}
	return []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".claude", "local"),
	}
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return executableMode(info)
}
