package nav

import (
	"encoding/csv"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FailedCorrigendumWriter writes failed website logs for corrigendums
type FailedCorrigendumWriter struct {
	file   *os.File
	writer *csv.Writer
	mu     sync.Mutex
}

// NewFailedCorrigendumWriter creates a writer under TenderData/Failed/Corrigendums/today_date/corrs.csv
func NewFailedCorrigendumWriter() *FailedCorrigendumWriter {
	today := time.Now().Format("02_Jan_2006")
	dir := filepath.Join("TenderData", "Failed", "Corrigendums", today)

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		log.Fatalf("failed to create directories for failed corrigendums: %v", err)
	}

	filePath := filepath.Join(dir, "corrs.csv")
	file, err := os.Create(filePath)
	if err != nil {
		log.Fatalf("failed to create failed corrigendums CSV: %v", err)
	}

	writer := csv.NewWriter(file)

	// Write header
	if err := writer.Write([]string{"State", "Error"}); err != nil {
		log.Printf("failed to write header: %v", err)
	}

	return &FailedCorrigendumWriter{
		file:   file,
		writer: writer,
	}
}

// WriteFailure writes a failed state and its error immediately
func (fw *FailedCorrigendumWriter) WriteFailure(state, errMsg string) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if err := fw.writer.Write([]string{state, errMsg}); err != nil {
		log.Printf("[%s] failed to write failure: %v", state, err)
	}
	fw.writer.Flush()
}

// Close closes the CSV file
func (fw *FailedCorrigendumWriter) Close() {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	fw.writer.Flush()
	if err := fw.file.Close(); err != nil {
		log.Printf("failed to close failed corrigendums file: %v", err)
	}
}
