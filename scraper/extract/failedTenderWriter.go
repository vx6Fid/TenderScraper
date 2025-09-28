package extract

import (
	"encoding/csv"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FailedTenderWriter writes failed tender links per state
type FailedTenderWriter struct {
	file   *os.File
	writer *csv.Writer
	mu     sync.Mutex
}

// NewFailedTenderWriter creates a CSV writer under TenderData/Failed/TenderInfo/today/state.csv
func NewFailedTenderWriter(state string) *FailedTenderWriter {
	today := time.Now().Format("02_Jan_2006")
	dir := filepath.Join("TenderData", "Failed", "TenderInfo", today)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("failed to create directories for failed tenders: %v", err)
	}

	filePath := filepath.Join(dir, state+".csv")
	file, err := os.Create(filePath)
	if err != nil {
		log.Fatalf("failed to create failed tenders CSV: %v", err)
	}

	writer := csv.NewWriter(file)
	if err := writer.Write([]string{"Serial", "Link", "Error"}); err != nil {
		log.Printf("failed to write header for %s: %v", state, err)
	}

	return &FailedTenderWriter{
		file:   file,
		writer: writer,
	}
}

// WriteFailure writes a failed tender record (thread-safe)
func (fw *FailedTenderWriter) WriteFailure(serial, link, errMsg string) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if err := fw.writer.Write([]string{serial, link, errMsg}); err != nil {
		log.Printf("[%s] failed to write failure: %v", serial, err)
	}
	fw.writer.Flush()
}

// Close closes the CSV file safely
func (fw *FailedTenderWriter) Close() {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	fw.writer.Flush()
	if err := fw.file.Close(); err != nil {
		log.Printf("failed to close failed tenders file: %v", err)
	}
}
