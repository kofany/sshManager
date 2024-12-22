package utils

import (
	"runtime"
	"strings"
)

func IsUNCPath(path string) bool {
	return runtime.GOOS == "windows" && strings.HasPrefix(path, "\\\\")
}

func PreserveUNCPath(path string) string {
	if IsUNCPath(path) {
		return "\\" + NormalizePath(path[1:], false)
	}
	return NormalizePath(path, false)
}

func NormalizePath(path string, isRemote bool) string {
	// Usuń potencjalne znaki "~" na początku ścieżki
	path = strings.TrimPrefix(path, "~")

	// Konwertuj separatory zgodnie z docelowym systemem
	if isRemote {
		// Dla zdalnego systemu zawsze używaj forward slash
		path = strings.ReplaceAll(path, "\\", "/")
		// Usuń niepotrzebne separatory na początku
		path = strings.TrimPrefix(path, "/")
	} else if runtime.GOOS == "windows" {
		// Dla lokalnego Windows konwertuj na backslash
		path = strings.ReplaceAll(path, "/", "\\")
		// Usuń niepotrzebne separatory na początku
		path = strings.TrimPrefix(path, "\\")
	}

	// Wyczyść podwójne separatory
	if runtime.GOOS == "windows" && !isRemote {
		path = strings.ReplaceAll(path, "\\\\", "\\")
	} else {
		path = strings.ReplaceAll(path, "//", "/")
	}

	return path
}
