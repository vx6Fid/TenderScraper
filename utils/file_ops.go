package utils

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/vx6fid/tender-scraper/utils/types"
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

// SaveToCSV writes scraped tenders to CSV
func SaveToCSV(tenders []types.Tender, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"ID", "Title", "Authority", "Submission Start Date", "Submission End Date"})
	for _, tender := range tenders {
		err := writer.Write([]string{
			tender.BasicDetails.TenderID,
			tender.WorkItemDetails.Title,
			tender.BasicDetails.OrganisationChain,
			tender.CriticalDates.BidSubmissionStartDate,
			tender.CriticalDates.BidSubmissionEndDate,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// AppendJSONL appends a single struct as JSON line
func AppendJSONL(filename string, v any) error {
	if err := os.MkdirAll(dirname(filename), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	bw := bufio.NewWriter(f)
	enc := json.NewEncoder(bw)
	if err := enc.Encode(v); err != nil {
		return err
	}
	return bw.Flush()
}

func dirname(path string) string {
	return filepath.Dir(path)
}
