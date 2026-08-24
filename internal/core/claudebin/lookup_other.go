//go:build !windows

package claudebin

import "os"

func binaries() []string {
	return []string{"claude"}
}

func dirs() []string {
	return append(homeDirs(), "/opt/homebrew/bin", "/usr/local/bin")
}

func executableMode(info os.FileInfo) bool {
	return info.Mode().Perm()&0o111 != 0
}
