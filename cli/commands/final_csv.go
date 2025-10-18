package commands

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/vx6fid/tender-scraper/utils"
)

// Data structure for final output
type TenderF struct {
	TenderID     string
	Title        string
	EPublished   string
	Organisation string
	ClosingDate  string
	Link         string
	UniqueID     string
}

func PrepareFinalCSV(logger *log.Logger) error {
	runDate := utils.GetRunDate(false)
	// runDate := "01_Oct_2025"

	var dataDir string
	flag.StringVar(&dataDir, "data-dir", "TenderData", "Path to TenderData directory")
	flag.Parse()

	for _, u := range utils.BaseURLs {
		commonDir := filepath.Join(dataDir, "Links", runDate, u.State)
		if err := FinalCSV(runDate, commonDir); err != nil {
			logger.Printf("[%s] CSV generation failed: %v", u.State, err)
		} else {
			logger.Printf("[%s] CSV file generated successfully", u.State)
		}
	}

	return nil
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
			UniqueID:     row[8],
		}
		data[t.UniqueID] = t
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
		tenderID := utils.ExtractTenderID(row[6])
		t := TenderF{
			TenderID:     tenderID,
			Title:        row[5],
			EPublished:   row[2],
			Organisation: row[8],
			ClosingDate:  row[3],
			Link:         row[7],
			UniqueID:     row[9],
		}
		// Proper duplicate check
		if _, found := data[t.UniqueID]; found {
			fmt.Printf("Duplicate found for UniqueID %s:\n", t.UniqueID)
		}

		data[t.UniqueID] = t
	}
	return data, nil
}

func FinalCSV(state, commonDir string) error {
	// Inputs
	corrigendumPath := filepath.Join(commonDir, "corrigendums.csv")
	searchPath := filepath.Join(commonDir, "search.csv")
	outputPath := filepath.Join(commonDir, "FinalLinks.csv")

	// fmt.Println("Paths: ", corrigendumPath, searchPath, outputPath)
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
	fmt.Println("Paths:", corrigendumPath, searchPath)
	fmt.Println("Length of Corrigendums:", len(corrData))
	fmt.Println("Length of Search Results:", len(searchData))

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
