package utils

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Data structure for final output
type TenderF struct {
	TenderID     string
	Title        string
	EPublished   string
	Organisation string
	ClosingDate  string
	Link         string
}

// Extract last [...] as Tender ID from search.csv TenderID field
func ExtractTenderID(s string) string {
	re := regexp.MustCompile(`\[(.*?)\]`)
	matches := re.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(matches[len(matches)-1][1])
}

func readCorrigendum(path string) (map[string]TenderF, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.LazyQuotes = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	data := make(map[string]TenderF)
	for i, row := range records {
		if i == 0 { // skip header
			continue
		}
		t := TenderF{
			TenderID:     row[1],
			Title:        row[3],
			EPublished:   row[4],
			Organisation: row[5],
			ClosingDate:  row[6],
			Link:         row[7],
		}
		data[t.TenderID] = t
	}
	return data, nil
}

func readSearch(path string) (map[string]TenderF, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.LazyQuotes = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	data := make(map[string]TenderF)
	for i, row := range records {
		if i == 0 { // skip header
			continue
		}
		tenderID := ExtractTenderID(row[6])
		t := TenderF{
			TenderID:     tenderID,
			Title:        row[5],
			EPublished:   row[2],
			Organisation: row[8],
			ClosingDate:  row[3],
			Link:         row[7],
		}
		data[t.TenderID] = t
	}
	return data, nil
}

func FinalCSV(runDate string, state string) error {
	// Output path
	commonDir := filepath.Join("TenderData", "Links", runDate, state)
	if err := os.MkdirAll(commonDir, 0755); err != nil {
		return err
	}

	// Inputs
	corrigendumPath := filepath.Join(commonDir, "corrigendums.csv")
	searchPath := filepath.Join(commonDir, "search.csv")
	outputPath := filepath.Join(commonDir, "FinalLinks.csv")

	// Check if files exist
	_, corrErr := os.Stat(corrigendumPath)
	_, searchErr := os.Stat(searchPath)

	if os.IsNotExist(corrErr) && os.IsNotExist(searchErr) {
		return fmt.Errorf("[%s] Failed to find input files", state)
	}

	// Read both files safely
	corrData := make(map[string]TenderF)
	if corrErr == nil {
		var err error
		corrData, err = readCorrigendum(corrigendumPath)
		if err != nil {
			panic(err)
		}
	}

	searchData := make(map[string]TenderF)
	if searchErr == nil {
		var err error
		searchData, err = readSearch(searchPath)
		if err != nil {
			panic(err)
		}
	}

	// Merge with corrigendum taking precedence
	merged := make(map[string]TenderF)
	for id, t := range searchData {
		merged[id] = t
	}
	for id, t := range corrData {
		merged[id] = t // overwrite if exists
	}

	// If nothing to write, skip
	if len(merged) == 0 {
		fmt.Printf("No records to write in %s, skipping.\n", commonDir)
		return fmt.Errorf("No records to write in %s", commonDir)
	}

	// Write final CSV
	file, err := os.Create(outputPath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{"Serial Number", "Tender ID", "Title", "E-Published Date", "Organisation", "Closing Date", "Link"}
	if err := writer.Write(header); err != nil {
		panic(err)
	}

	i := 1
	for _, t := range merged {
		record := []string{
			fmt.Sprintf("%d", i),
			t.TenderID,
			t.Title,
			t.EPublished,
			t.Organisation,
			t.ClosingDate,
			t.Link,
		}
		if err := writer.Write(record); err != nil {
			panic(err)
		}
		i++
	}
	return nil
}
