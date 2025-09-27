package nav

import (
	"encoding/csv"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type PastWriter struct {
	rows chan []string
	wg   sync.WaitGroup
}

func NewPastWriter(state string, headers []string) *PastWriter {
	today := time.Now().Format("02_Jan_2006")
	dir := filepath.Join("TenderData", "PastLinks", today)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		log.Fatalf("failed to create directories: %v", err)
	}

	filePath := filepath.Join(dir, state+".csv")
	file, err := os.Create(filePath)
	if err != nil {
		log.Fatalf("failed to create CSV: %v", err)
	}

	w := csv.NewWriter(file)
	pw := &PastWriter{rows: make(chan []string, 2000)}
	pw.wg.Add(1)

	go func() {
		defer pw.wg.Done()
		defer file.Close()
		defer w.Flush()

		// Write custom headers
		if len(headers) > 0 {
			if err := w.Write(headers); err != nil {
				log.Printf("header write error: %v", err)
			}
		}

		// Write data rows
		for row := range pw.rows {
			if err := w.Write(row); err != nil {
				log.Printf("write error: %v", err)
			}
		}
	}()

	return pw
}

func (pw *PastWriter) WriteRow(row []string) {
	pw.rows <- row
}

func (pw *PastWriter) Close() {
	close(pw.rows)
	pw.wg.Wait()
}
