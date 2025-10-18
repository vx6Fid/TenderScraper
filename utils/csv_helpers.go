package utils

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
)

// EstimateJobCount reads a CSV and counts valid rows
func EstimateJobCount(state, runDate string, pastTenders bool, stage string) (int, error) {
	fileName := "search.csv"
	filePath := fmt.Sprintf("TenderData/Links/%s/%s", runDate, state)
	if pastTenders {
		fileName = fmt.Sprintf("%s_%s.csv", state, stage)
		filePath = fmt.Sprintf("TenderData/PastLinks/%s", runDate)
	}
	inputPath := filepath.Join(filePath, fileName)

	inFile, err := os.Open(inputPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open links CSV at %s: %w", inputPath, err)
	}
	defer inFile.Close()

	reader := csv.NewReader(inFile)
	rows, err := reader.ReadAll()
	if err != nil {
		return 0, fmt.Errorf("failed to read links CSV: %w", err)
	}
	if len(rows) < 1 {
		return 0, fmt.Errorf("no data rows found in %s", inputPath)
	}

	totalJobs := 0
	for i := 1; i < len(rows); i++ {
		if len(rows[i]) >= 5 {
			totalJobs++
		}
	}
	return totalJobs, nil
}
