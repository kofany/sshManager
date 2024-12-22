package utils

import (
	"runtime"
	"strings"
)

// ToSFTPPath converts local path to SFTP path format
func ToSFTPPath(path string) string {
	if runtime.GOOS == "windows" {
		return strings.ReplaceAll(path, "\\", "/")
	}
	return path
}

// ToLocalPath converts SFTP path to local path format
func ToLocalPath(path string) string {
	if runtime.GOOS == "windows" {
		return strings.ReplaceAll(path, "/", "\\")
	}
	return path
}
