package utils

import (
	"os"
	"path/filepath"
)

// SaveToFile writes byte content to a file
func SaveToFile(content []byte, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(content)
	return err
}

func dirname(path string) string {
	return filepath.Dir(path)
}
