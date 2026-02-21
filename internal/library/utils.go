package library

import (
	"os"
	"path/filepath"
	"slices"
)

var (
	VALID_EXT = []string{".mkv", ".mp4", ".avi"}
)

func IsValidVideoFile(name string) bool {
	ext := filepath.Ext(name)
	return slices.Contains(VALID_EXT, ext)
}

func IsFolderExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
