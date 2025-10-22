package active

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Tender struct {
	Serial           int
	Title            string
	Organisation     string
	PublishedDate    string
	ClosingDate      string
	Link             string
	UniqueIdentifier string
}

func IsInvalidTender(t Tender) bool {
	return t.Title == "" || t.Organisation == "" || t.ClosingDate == "" || t.Link == "" || t.UniqueIdentifier == ""
}

func ExtractTitle(raw string) string {
	re := regexp.MustCompile(`\[(.*?)\]`) // match anything inside [ ]
	matches := re.FindStringSubmatch(raw)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return strings.TrimSpace(raw) // fallback if no brackets found
}

func SaveTendersCSVBatch(state string, tenders []Tender, isFirstBatch bool) error {
	dateStr := time.Now().Format("02_Jan_2006")
	dir := filepath.Join("TenderData", "Links", dateStr, state)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	filePath := filepath.Join(dir, "active.csv")
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open csv file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// write header only for the first batch
	if isFirstBatch {
		//Clear the file before writing header
		if err := os.Truncate(filePath, 0); err != nil {
			return fmt.Errorf("failed to clear file: %w", err)
		}

		if err := writer.Write([]string{"Serial", "Title", "Organisation", "e-Published Date", "Closing Date", "Link", "Unique Identifier"}); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
	}

	for _, t := range tenders {
		if err := writer.Write([]string{
			strconv.Itoa(t.Serial),
			t.Title,
			t.Organisation,
			t.PublishedDate,
			t.ClosingDate,
			t.Link,
			t.UniqueIdentifier,
		}); err != nil {
			return fmt.Errorf("failed to write tender data: %w", err)
		}
	}

	// log.Printf("Saved %d tenders to %s", len(tenders), filePath)
	return nil
}
