package files

import (
	"fmt"
	"os"
)

func FileExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

func CreateDir(dirPath string) error {
	err := os.MkdirAll(dirPath, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return nil
}

func DeleteDir(dirPath string) error {
	err := os.RemoveAll(dirPath)
	if err != nil {
		return fmt.Errorf("failed to delete directory: %w", err)
	}
	return nil
}
