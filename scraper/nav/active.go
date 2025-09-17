package nav

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/vx6fid/tender-scraper/session"
)

// TenderScraper holds the state and configuration for the scraping process.
type TenderScraper struct {
	collector          *colly.Collector
	baseURL            string
	state              string
	captchaSolved      bool
	sessionEstablished bool
	resultsFound       bool
	activeTendersURL   string
	totalTenders       int
	scrapedTenders     int
	currentPage        int
	nextButtonURL      string
	logger             *log.Logger
	csvFile            *os.File
	csvWriter          *csv.Writer
}

// NewActiveScraper initializes a new scraper instance.
func NewActiveScraper(sess *session.Session, domain string, state string) *TenderScraper {
	collector := sess.NewCollector(domain)
	collector.AllowURLRevisit = true
	return &TenderScraper{
		collector:        collector,
		baseURL:          sess.BaseURL,
		state:            state,
		activeTendersURL: sess.ActiveTendersURL,
		currentPage:      1,
	}
}

// Main entry point to start the scraping process
func (ts *TenderScraper) ScrapeActiveTenders() error {
	// file to save log of the scraping process
	logFileName := fmt.Sprintf("TenderData/Logs/collectors/%s_%s.txt",
		ts.state, time.Now().Format("02_Jan_2006_15_04_05"))

	logFile, err := os.Create(logFileName)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()

	// initialize logger
	ts.logger = log.New(logFile, "", log.LstdFlags)

	ts.logger.Println("Starting tender scraping process with correct session flow.")

	dateStr := time.Now().Format("02_Jan_2006")
	dir := filepath.Join("TenderData/Links", dateStr)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fileName := fmt.Sprintf("%s_Links.csv", ts.state)
	filePath := filepath.Join(dir, fileName)
	// open CSV, save file to TenderLinks folder
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	ts.csvFile = file
	ts.csvWriter = csv.NewWriter(file)

	// write header
	ts.csvWriter.Write([]string{"Serial Number", "Title", "Organisation", "Closing Date", "Link"})
	ts.csvWriter.Flush()

	defer func() {
		if ts.csvWriter != nil {
			ts.csvWriter.Flush()
		}
		if ts.csvFile != nil {
			ts.csvFile.Close()
		}
	}()

	// STEP 1: Click Active Tenders link with established session
	ts.logger.Printf("STEP 1: Clicking Active Tenders link with session: %s", ts.activeTendersURL)

	// Clear handlers and setup for tender parsing
	ts.clearHandlers()
	ts.setupTenderHandlers()

	if err := ts.collector.Visit(ts.activeTendersURL); err != nil {
		return fmt.Errorf("failed to visit active tenders page with session: %w", err)
	}

	if !ts.resultsFound {
		return fmt.Errorf("no tender results found after establishing session")
	}

	// STEP 2: Handle pagination - keep clicking "Next" until no more pages
	ts.logger.Println("STEP 2: Starting pagination process...")
	if err := ts.handlePagination(); err != nil {
		return fmt.Errorf("pagination failed: %w", err)
	}

	ts.logger.Printf("Scraping completed successfully! Total tenders scraped: %d/%d", ts.scrapedTenders, ts.totalTenders)
	return nil
}

// handlePagination processes all pages by clicking Next button
func (ts *TenderScraper) handlePagination() error {
	for ts.nextButtonURL != "" {
		// stop if scraped enough tenders
		if ts.totalTenders > 0 && ts.scrapedTenders >= ts.totalTenders {
			ts.logger.Printf("All %d tenders scraped, stopping pagination", ts.scrapedTenders)
			break
		}

		ts.logger.Printf("PAGE %d: Clicking Next button: %s", ts.currentPage+1, ts.nextButtonURL)

		// Small delay between page requests
		time.Sleep(500 * time.Millisecond)

		// Visit the next page
		if err := ts.collector.Visit(ts.nextButtonURL); err != nil {
			ts.logger.Printf("ERROR: Failed to visit next page: %v", err)
			break
		}

		ts.currentPage++

		// Log progress
		if ts.totalTenders > 0 {
			progress := float64(ts.scrapedTenders) / float64(ts.totalTenders) * 100
			ts.logger.Printf("Progress: %d/%d tenders (%.1f%%) - Page %d completed",
				ts.scrapedTenders, ts.totalTenders, progress, ts.currentPage)
		}
	}

	ts.logger.Printf("Pagination completed! Processed %d pages, scraped %d tenders", ts.currentPage, ts.scrapedTenders)
	return nil
}

// setupTenderHandlers configures handlers for tender scraping phase
func (ts *TenderScraper) setupTenderHandlers() {
	ts.collector.OnHTML("table#table", ts.handleTenderTable)
	ts.collector.OnHTML("a#loadNext", ts.handleNextButton)
	ts.collector.OnHTML("span:contains('Total records:')", ts.handleTotalRecords)
	ts.collector.OnError(ts.handleError)
}

// clearHandlers removes all existing handlers
func (ts *TenderScraper) clearHandlers() {
	oldCollector := ts.collector
	ts.collector = ts.collector.Clone()

	// Copy cookies from old collector for baseURL
	cookies := oldCollector.Cookies(ts.baseURL)
	if len(cookies) > 0 {
		ts.collector.SetCookies(ts.baseURL, cookies)
	}
}

// handleError handles errors during scraping
func (ts *TenderScraper) handleError(r *colly.Response, err error) {
	ts.logger.Printf("ERROR: Request failed - URL: %s, Status: %d, Error: %v",
		r.Request.URL, r.StatusCode, err)
}

// handleTenderTable processes the tender results table (Step 3)
func (ts *TenderScraper) handleTenderTable(e *colly.HTMLElement) {
	ts.logger.Printf("PAGE %d: Found tender table! Parsing results... (Status: %d)", ts.currentPage, e.Response.StatusCode)
	ts.resultsFound = true
	ts.sessionEstablished = true
	ts.parseTenders(e)
}

// Test it for a small number of tenders to see it stops after scraping whole page
func (ts *TenderScraper) handleNextButton(e *colly.HTMLElement) {
	href := e.Attr("href")
	if href == "" {
		ts.nextButtonURL = ""
		ts.logger.Printf("PAGE %d: No more Next button found - reached last page", ts.currentPage)
		return
	}

	base, err := url.Parse(e.Request.URL.String())
	if err != nil {
		ts.logger.Printf("ERROR: Failed to parse base URL: %v", err)
		ts.nextButtonURL = href
		return
	}

	rel, err := url.Parse(href)
	if err != nil {
		ts.logger.Printf("ERROR: Failed to parse href: %v", err)
		ts.nextButtonURL = href
		return
	}

	ts.nextButtonURL = base.ResolveReference(rel).String()
	ts.logger.Printf("PAGE %d: Next button found: %s", ts.currentPage, ts.nextButtonURL)
}

// handleTotalRecords extracts the total records count
func (ts *TenderScraper) handleTotalRecords(e *colly.HTMLElement) {
	text := e.Text
	// Extract number from "Total records: 8174"
	re := regexp.MustCompile(`Total records:\s*(\d+)`)
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		if total, err := strconv.Atoi(matches[1]); err == nil {
			if ts.totalTenders == 0 { // Only set on first page
				ts.totalTenders = total
				ts.logger.Printf("Total records found: %d", ts.totalTenders)
			}
		}
	}
}

// parseTenders parses tender data from HTML element
func (ts *TenderScraper) parseTenders(e *colly.HTMLElement) {
	ts.logger.Printf("PAGE %d: Parsing tender data from table element...", ts.currentPage)
	var tendersFoundOnPage int
	// ts.saveFile("debug", fmt.Sprintf("Page_%d.html", ts.currentPage), []byte(e.Response.Body))

	// e.DOM.Find("tr.even, tr.odd").Each(func(i int, s *goquery.Selection) {
	e.DOM.Find(`tr[id^="informal"]`).Each(func(i int, s *goquery.Selection) {
		tendersFoundOnPage++
		ts.scrapedTenders++
		cells := s.Find("td")

		if cells.Length() >= 6 {
			closingDate := strings.TrimSpace(cells.Eq(2).Text())

			linkTag := cells.Eq(4).Find("a")
			title := strings.TrimSpace(linkTag.Text())
			href, exists := linkTag.Attr("href")
			if !exists {
				href = "" // no link present
			}

			organisation := strings.TrimSpace(cells.Eq(5).Text())

			// Print tender details to screen
			// fmt.Printf("TENDER #%d (Page %d, Item %d):\n", ts.scrapedTenders, ts.currentPage, tendersFoundOnPage)
			// fmt.Printf("  Title: %s\n", title)
			// fmt.Printf("  Organization: %s\n", organisation)
			// fmt.Printf("  Closing Date: %s\n", closingDate)
			// fmt.Printf("  Link: %s\n", href)
			// fmt.Println("  " + strings.Repeat("-", 50))

			fullLink := href
			if href != "" {
				base, err := url.Parse(ts.baseURL)
				if err == nil {
					rel, err := url.Parse(href)
					if err == nil {
						fullLink = base.ResolveReference(rel).String()
					}
				}
			}
			if err := ts.csvWriter.Write([]string{
				strconv.Itoa(ts.scrapedTenders),
				title,
				organisation,
				closingDate,
				fullLink,
			}); err != nil {
				ts.logger.Printf("ERROR: Failed to write CSV row: %v", err)
			}

			ts.logger.Printf("  Tender %d (Page %d): '%s' ", ts.scrapedTenders, ts.currentPage, title)
		}

	})

	ts.csvWriter.Flush()

	ts.logger.Printf("PAGE %d: Successfully parsed %d tenders", ts.currentPage, tendersFoundOnPage)
}

// saveFile utility function
func (ts *TenderScraper) saveFile(dir, filename string, body []byte) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("could not create directory %s: %w", dir, err)
	}

	fullPath := filepath.Join(dir, filename)
	if err := os.WriteFile(fullPath, body, 0644); err != nil {
		return fmt.Errorf("could not write file %s: %w", fullPath, err)
	}

	ts.logger.Printf("Saved: %s", fullPath)
	return nil
}
