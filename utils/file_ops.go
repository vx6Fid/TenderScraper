package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnsureDir creates the directory (and parents) if it does not already exist.
func EnsureDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create dir %q: %w", dir, err)
	}
	return nil
}

// EnsureParentDir creates the parent directory of a file path if missing.
func EnsureParentDir(filePath string) error {
	return EnsureDir(filepath.Dir(filePath))
}

// CreateFile is like os.Create but first ensures the parent directory exists.
// Use this instead of os.Create for any path under a directory that may not
// have been created yet.
func CreateFile(filePath string) (*os.File, error) {
	if err := EnsureParentDir(filePath); err != nil {
		return nil, err
	}
	return os.Create(filePath)
}

// SaveToFile writes byte content to a file, creating parent dirs as needed.
func SaveToFile(content []byte, filename string) error {
	file, err := CreateFile(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(content)
	return err
}
