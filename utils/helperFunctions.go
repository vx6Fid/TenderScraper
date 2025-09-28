package utils

import (
	"bufio"
	"context"
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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gocolly/colly/v2"
	"github.com/vx6fid/tender-scraper/session"
)

// GetDateRange returns the start and end dates for the past tender data
func GetDateRange() (string, string) {
	// read from and to date
	fmt.Println("Enter start date(DD/MM/YYYY):")
	var from string
	fmt.Scanln(&from)

	fmt.Println("Enter end date(DD/MM/YYYY):")
	var to string
	fmt.Scanln(&to)

	return from, to
}

// CheckTenderFolderExists checks if the given tenderID folder exists in the S3 bucket
func CheckTenderFolderExists(bucket, tenderID string) (bool, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return false, fmt.Errorf("unable to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg)

	// Construct prefix for the tender folder
	prefix := fmt.Sprintf("tender-documents/%s/", tenderID)

	// List objects with that prefix
	resp, err := client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(1), // optimization, we only care if at least one exists
	})
	if err != nil {
		return false, fmt.Errorf("failed to list objects: %w", err)
	}

	// If no contents, folder doesn't exist
	if len(resp.Contents) == 0 {
		return false, nil
	}

	return true, nil
}

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

func GetDomain(baseURL string) (string, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	return parsedURL.Host, nil
}

// GetBaseURLAndState finds the baseURL and state for a given tenderURL
func GetBaseURLAndState(tenderURL string) (string, string, error) {
	parsed, err := url.Parse(tenderURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid tenderURL: %w", err)
	}

	host := parsed.Hostname()

	for _, entry := range BaseURLs {
		if strings.EqualFold(host, entry.Domain) {
			return entry.BaseURL, entry.State, nil
		}
	}

	return "", "", fmt.Errorf("no matching baseURL found for host %s", host)
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
func GetRunDate(PastTenders bool) string {
	dirPath := "TenderData/Links"
	if PastTenders {
		dirPath = "TenderData/PastLinks"
	}

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

func EstimateJobCount(state string, runDate string, PastTenders bool) (int, error) {
	fileName := "FinalLinks.csv"
	filePath := fmt.Sprintf("TenderData/Links/%s/%s", runDate, state)
	if PastTenders {
		fileName = fmt.Sprintf("%s.csv", state)
		filePath = fmt.Sprintf("TenderData/PastLinks/%s", runDate)
	}
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

type seenInfo struct {
	rowNum int
	record []string
}

func ShowDuplicatesByIDLink(filePath string, outFile string) {
	f, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	rows, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("Failed to read CSV: %v", err)
	}
	if len(rows) < 2 {
		log.Fatalf("CSV has no data rows")
	}

	header := rows[0]
	fmt.Println("Header:", header)

	// open output log file
	out, err := os.Create(outFile)
	if err != nil {
		log.Fatalf("Failed to create output file: %v", err)
	}
	defer out.Close()

	// writer for duplicates
	logger := log.New(out, "", 0)

	// figure out index of TenderID and Link
	// var idxTenderID int = -1
	var idxLink int = -1
	for i, h := range header {
		// if h == "TenderID" {
		// 	idxTenderID = i
		// }
		if h == "Link" {
			idxLink = i
		}
	}
	// if idxTenderID == -1 || idxLink == -1 {
	if idxLink == -1 {
		log.Fatalf("CSV missing TenderID or Link column")
	}

	// seenTenderID := make(map[string]seenInfo)
	seenLink := make(map[string]seenInfo)

	// process rows
	for i, row := range rows[0:] {
		rowNum := i + 1
		// tid := row[idxTenderID]
		link := row[idxLink]

		// check TenderID
		// if prev, ok := seenTenderID[tid]; ok {
		// 	logger.Printf("[DUP TenderID] Row %d duplicates Row %d → TenderID=%s\n", rowNum, prev.rowNum, tid)
		// 	logger.Printf("  Previous: %v\n", prev.record)
		// 	logger.Printf("  Current : %v\n", row)
		// } else {
		// 	seenTenderID[tid] = seenInfo{rowNum, row}
		// }

		// check Link
		if prev, ok := seenLink[link]; ok {
			logger.Printf("[DUP Link] Row %d duplicates Row %d → Link=%s\n", rowNum, prev.rowNum, link)
			logger.Printf("  Previous: %v\n", prev.record)
			logger.Printf("  Current : %v\n", row)
		} else {
			seenLink[link] = seenInfo{rowNum, row}
		}
	}
}

func ShowDuplicatesByRow(filePath string, outFile string) {
	f, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	rows, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("Failed to read CSV: %v", err)
	}
	if len(rows) < 2 {
		log.Fatalf("CSV has no data rows")
	}

	header := rows[0]
	fmt.Println("Header:", header)

	out, err := os.Create(outFile)
	if err != nil {
		log.Fatalf("Failed to create output file: %v", err)
	}
	defer out.Close()

	logger := log.New(out, "", 0)

	// find TenderID and Link columns
	idxTenderID, idxLink := -1, -1
	for i, h := range header {
		if h == "TenderID" {
			idxTenderID = i
		}
		if h == "Link" {
			idxLink = i
		}
	}
	if idxTenderID == -1 || idxLink == -1 {
		log.Fatalf("CSV missing TenderID or Link column")
	}

	seen := make(map[string]seenInfo)

	for i, row := range rows[1:] {
		rowNum := i + 2

		if len(row) <= idxLink {
			continue
		}

		tid := strings.TrimSpace(row[idxTenderID])
		link := strings.TrimSpace(row[idxLink])
		key := tid + "|" + link

		if key == "|" {
			continue // skip blank
		}

		if prev, ok := seen[key]; ok {
			logger.Printf("[DUP] Row %d duplicates Row %d → TenderID=%s Link=%s\n", rowNum, prev.rowNum, tid, link)
			logger.Printf("  Previous: %v\n", prev.record)
			logger.Printf("  Current : %v\n", row)
			continue
		}

		seen[key] = seenInfo{rowNum, row}
	}
}
