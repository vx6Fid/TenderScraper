package nav

import (
	"encoding/csv"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type CSVWriter struct {
	rows chan []string
	wg   sync.WaitGroup
}

func NewCSVWriter(state string) *CSVWriter {
	today := time.Now().Format("02_Jan_2006")
	dir := filepath.Join("TenderData", "Links", today, state)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		log.Fatalf("failed to create directories: %v", err)
	}

	filePath := filepath.Join(dir, "search.csv")
	file, err := os.Create(filePath)
	if err != nil {
		log.Fatalf("failed to create CSV: %v", err)
	}

	w := csv.NewWriter(file)
	cw := &CSVWriter{rows: make(chan []string, 4000)}

	cw.wg.Add(1)
	go func() {
		defer cw.wg.Done()
		defer file.Close()
		defer w.Flush()

		// header
		w.Write([]string{"S.No", "Page No", "e-Published Date", "Closing Date", "Opening Date",
			"Title", "TenderID", "Link", "Organisation Chain", "Unique Identifier"})

		for row := range cw.rows {
			if err := w.Write(row); err != nil {
				log.Printf("write error: %v", err)
			}
		}
	}()

	return cw
}

func (cw *CSVWriter) WriteRow(row []string) {
	cw.rows <- row
}

func (cw *CSVWriter) Close() {
	close(cw.rows)
	cw.wg.Wait()
}
