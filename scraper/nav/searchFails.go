package nav

import (
	"encoding/csv"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type FailedSearchWriter struct {
	rows     chan []string
	wg       sync.WaitGroup
	filePath string
}

func NewFailedSearchWriter(state string) *FailedSearchWriter {
	today := time.Now().Format("02_Jan_2006")
	dir := filepath.Join("TenderData", "Failed", "SearchTenders", today)
	os.MkdirAll(dir, os.ModePerm)

	filePath := filepath.Join(dir, state+".csv")
	file, err := os.Create(filePath)
	if err != nil {
		log.Fatalf("failed to create Failed CSV: %v", err)
	}

	w := csv.NewWriter(file)
	fw := &FailedSearchWriter{
		rows:     make(chan []string, 1000),
		filePath: filePath,
	}
	fw.wg.Add(1)
	go func() {
		defer fw.wg.Done()
		defer file.Close()
		defer w.Flush()

		// Write header
		w.Write([]string{"PageNumber", "Error"})

		for row := range fw.rows {
			w.Write(row)
			w.Flush() // immediate flush
		}
	}()
	return fw
}

func (fw *FailedSearchWriter) WriteFailure(pageNum int, errMsg string) {
	fw.rows <- []string{strconv.Itoa(pageNum), errMsg}
}

func (fw *FailedSearchWriter) Close() {
	close(fw.rows)
	fw.wg.Wait()
}
