package utils

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gocolly/colly/v2"
	"github.com/vx6fid/tender-scraper/session"
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
	return min(totalJobs, 120)
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

func EstimateJobCount(state string, runDate string) (int, error) {
	fileName := fmt.Sprintf("%s_Links.csv", state)
	filePath := fmt.Sprintf("TenderData/Links/%s", runDate)
	inputPath := filepath.Join(filePath, fileName)

	// Open the CSV file
	inFile, err := os.Open(inputPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open links CSV at %s: %w", inputPath, err)
	}
	defer inFile.Close()

	// Read the CSV content
	reader := csv.NewReader(inFile)
	rows, err := reader.ReadAll()
	if err != nil {
		return 0, fmt.Errorf("failed to read links CSV: %w", err)
	}

	// Ensure there are data rows (at least 2 rows: header + data)
	if len(rows) <= 1 {
		return 0, fmt.Errorf("no data rows found in %s", inputPath)
	}

	// Count valid jobs (rows with at least 5 columns)
	totalJobs := 0
	for i := 1; i < len(rows); i++ { // Skip the header row (i = 1)
		if len(rows[i]) >= 5 {
			totalJobs++
		}
	}

	return totalJobs, nil
}

func FetchTotalPages(sess *session.Session, baseURL string, domain string) (int, error) {
	collector := sess.NewCollector(domain)
	var totalPages int
	var scrapeErr error

	collector.OnHTML("a#linkLast", func(e *colly.HTMLElement) {
		href := e.Attr("href")
		// fmt.Println("[TotalPages] ", href)
		// href looks like: "...&sp=AFrontEndAdvancedSearchResult%2Ctable&sp=399"
		u, err := url.Parse(href)
		if err != nil {
			scrapeErr = fmt.Errorf("parse href: %w", err)
			return
		}
		q := u.Query()
		spParams := q["sp"]
		if len(spParams) == 0 {
			scrapeErr = fmt.Errorf("no sp param found in href=%s", href)
			return
		}

		// Get the last sp parameter (which should contain the page number)
		lastSpParam := spParams[len(spParams)-1]

		// Parse the page number from the last sp parameter
		p, err := strconv.Atoi(lastSpParam)
		if err != nil {
			scrapeErr = fmt.Errorf("invalid page number %q: %w", lastSpParam, err)
			return
		}
		totalPages = p
	})

	searchPage1 := BuildPageURLRaw(baseURL, 1)
	err := collector.Visit(searchPage1)
	if err != nil {
		return 0, fmt.Errorf("visit failed: %w", err)
	}

	if scrapeErr != nil {
		return 0, scrapeErr
	}
	if totalPages == 0 {
		return 0, fmt.Errorf("could not find last page anchor")
	}
	return totalPages, nil
}

func BuildPageURLRaw(baseURL string, currentPage int) string {
	return fmt.Sprintf("%s?component=$TablePages.linkPage&page=FrontEndAdvancedSearchResult&service=direct&session=T&sp=AFrontEndAdvancedSearchResult%%2Ctable&sp=%d", baseURL, currentPage)
}

func CountUniqueTenderIDs(filePath string) error {
	// Open the CSV file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create CSV reader
	reader := csv.NewReader(file)

	// Read all records
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) == 0 {
		fmt.Println("CSV file is empty")
		return nil
	}

	// Find the tenderID column index
	headers := records[0]
	tenderIDIndex := -1

	for i, header := range headers {
		// Case-insensitive search for tenderID column
		if strings.ToLower(strings.TrimSpace(header)) == "tenderid" {
			tenderIDIndex = i
			break
		}
	}

	if tenderIDIndex == -1 {
		return fmt.Errorf("tenderID column not found in CSV headers: %v", headers)
	}

	// Use a map to store unique tender IDs
	uniqueTenderIDs := make(map[string]bool)

	// Process data rows (skip header)
	for i, record := range records[1:] {
		if len(record) <= tenderIDIndex {
			fmt.Printf("Warning: Row %d has insufficient columns, skipping\n", i+2)
			continue
		}

		tenderID := strings.TrimSpace(record[tenderIDIndex])
		if tenderID != "" {
			uniqueTenderIDs[tenderID] = true
		}
	}

	// Print results
	fmt.Printf("File: %s\n", filePath)
	fmt.Printf("Total rows (excluding header): %d\n", len(records)-1)
	fmt.Printf("Unique Tender IDs: %d\n", len(uniqueTenderIDs))

	return nil
}
