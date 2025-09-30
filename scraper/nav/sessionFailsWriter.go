package nav

import (
	"encoding/csv"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type FailedSessWriter struct {
	rows     chan []string
	wg       sync.WaitGroup
	filePath string
}

func NewFailedSessWriter() *FailedSessWriter {
	today := time.Now().Format("02_Jan_2006")
	dir := filepath.Join("TenderData", "Failed", "SearchTenders", today)
	os.MkdirAll(dir, os.ModePerm)

	filePath := filepath.Join(dir, "Failed_Sessions.csv")
	file, err := os.Create(filePath)
	if err != nil {
		log.Fatalf("failed to create Failed CSV: %v", err)
	}

	w := csv.NewWriter(file)
	fw := &FailedSessWriter{
		rows:     make(chan []string, 1000),
		filePath: filePath,
	}
	fw.wg.Add(1)
	go func() {
		defer fw.wg.Done()
		defer file.Close()
		defer w.Flush()

		// Write header
		w.Write([]string{"SateName", "BaseURL", "Error"})

		for row := range fw.rows {
			w.Write(row)
			w.Flush() // immediate flush
		}
	}()
	return fw
}

func (fw *FailedSessWriter) WriteFailure(state, baseURL, errMsg string) {
	fw.rows <- []string{state, baseURL, errMsg}
}

func (fw *FailedSessWriter) Close() {
	close(fw.rows)
	fw.wg.Wait()
}
