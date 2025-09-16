package utils

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"log"
	"os"
	"sort"
)

func SaveToFile(content []byte, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(content)
	if err != nil {
		return err
	}

	return nil
}

// SaveToCSV writes scraped tenders to a csv file
func SaveToCSV(tenders []Tender, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// header
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

// AppendJSONL appends a single struct value as one JSON line to filename.
// It creates the file and parent directories if they do not exist.
func AppendJSONL(filename string, v interface{}) error {
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
	last := -1
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			last = i
		}
	}
	if last <= 0 {
		return "."
	}
	return path[:last]
}

func CalculateOptimalWorkers(totalJobs int) int {
	return min(totalJobs, 96)
}

// function to get the last created folder name in TenderDate/Links
func GetRunDate() string {
	dirPath := "TenderData/Links"

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		log.Printf("Error reading directory: %v", err)
		return ""
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}

	if len(names) == 0 {
		return ""
	}

	sort.Strings(names)
	return names[len(names)-1] // last in sorted order
}

func EstimateJobCount(state string, runDate string) int {
	// Implement your logic here to estimate the job count based on state and runDate
	// This is just a placeholder implementation
	return 620
}
