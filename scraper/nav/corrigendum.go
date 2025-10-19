// Extract Tender ID, Page Number in each row

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
	"github.com/vx6fid/tender-scraper/utils"
)

// CorrScraper holds the state and configuration for the scraping process.
type CorrScraper struct {
	collector          *colly.Collector
	baseURL            string
	state              string
	captchaSolved      bool
	sessionEstablished bool
	resultsFound       bool
	corrigendumURL     string
	totalTenders       int
	scrapedTenders     int
	currentPage        int
	nextButtonURL      string
	logger             *log.Logger
	csvFile            *os.File
	csvWriter          *csv.Writer
	failedWriter       *FailedCorrigendumWriter
}

// NewCorrScraper initializes a new scraper instance.
func NewCorrScraper(sess *session.Session, domain string, state string, failed *FailedCorrigendumWriter) *CorrScraper {
	collector := sess.NewCollector(domain)
	collector.SetRequestTimeout(30 * time.Second)
	collector.AllowURLRevisit = true
	return &CorrScraper{
		collector:      collector,
		baseURL:        sess.BaseURL,
		state:          state,
		corrigendumURL: sess.CorrigendumURL,
		currentPage:    1,
		failedWriter:   failed,
	}
}

// Main entry point to start the scraping process
func (cs *CorrScraper) ScrapeCorrigendum() error {
	// file to save log of the scraping process
	logFileName := fmt.Sprintf("TenderData/Logs/collectors/%s_%s.txt",
		cs.state, time.Now().Format("02_Jan_2006_15_04_05"))

	logFile, err := os.Create(logFileName)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()

	// initialize logger
	cs.logger = log.New(logFile, "", log.LstdFlags)

	cs.logger.Println("Starting tender scraping process with correct session flow.")

	dateStr := time.Now().Format("02_Jan_2006")
	dir := fmt.Sprintf("TenderData/Links/%s/%s", dateStr, cs.state)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fileName := "corrigendums.csv"
	filePath := filepath.Join(dir, fileName)
	// open CSV, save file to TenderLinks folder
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	cs.csvFile = file
	cs.csvWriter = csv.NewWriter(file)

	// write header
	cs.csvWriter.Write([]string{
		"Serial Number",
		"Tender ID",
		"Page Number",
		"Title",
		"E-Published Date",
		"Organisation",
		"Closing Date",
		"Link",
		"UniqueIdentifier",
	})

	cs.csvWriter.Flush()

	defer func() {
		if cs.csvWriter != nil {
			cs.csvWriter.Flush()
		}
		if cs.csvFile != nil {
			cs.csvFile.Close()
		}
	}()

	// STEP 1: Click Corrigendum Tenders link with established session
	cs.logger.Printf("STEP 1: Clicking Corrigendum Tenders link with session: %s", cs.corrigendumURL)

	// Clear handlers and setup for tender parsing
	cs.clearHandlers()
	cs.setupTenderHandlers()

	if err := cs.collector.Visit(cs.corrigendumURL); err != nil {
		return fmt.Errorf("failed to visit corrigendum tenders page with session: %w", err)
	}

	cs.collector.Wait()

	if !cs.resultsFound {
		return fmt.Errorf("no tender results found after establishing session")
	}

	// STEP 2: Handle pagination - keep clicking "Next" until no more pages
	cs.logger.Println("STEP 2: Starting pagination process...")
	if err := cs.handlePagination(); err != nil {
		return fmt.Errorf("pagination failed: %w", err)
	}

	cs.logger.Printf("Scraping completed successfully! Total tenders scraped: %d/%d", cs.scrapedTenders, cs.totalTenders)
	return nil
}

// handlePagination processes all pages by clicking Next button
func (cs *CorrScraper) handlePagination() error {
	for cs.nextButtonURL != "" {
		// stop if scraped enough tenders
		if cs.totalTenders > 0 && cs.scrapedTenders >= cs.totalTenders {
			cs.logger.Printf("All %d tenders scraped, stopping pagination", cs.scrapedTenders)
			break
		}

		cs.logger.Printf("PAGE %d: Clicking Next button: %s", cs.currentPage+1, cs.nextButtonURL)

		// Small delay between page requests
		time.Sleep(200 * time.Millisecond)

		// Visit the next page
		if err := cs.collector.Visit(cs.nextButtonURL); err != nil {
			cs.logger.Printf("ERROR: Failed to visit next page: %v", err)
			return err
		}

		cs.currentPage++

		// Log progress
		if cs.totalTenders > 0 {
			progress := float64(cs.scrapedTenders) / float64(cs.totalTenders) * 100
			cs.logger.Printf("Progress: %d/%d tenders (%.1f%%) - Page %d completed",
				cs.scrapedTenders, cs.totalTenders, progress, cs.currentPage)
		}
	}

	cs.logger.Printf("Pagination completed! Processed %d pages, scraped %d tenders", cs.currentPage, cs.scrapedTenders)
	return nil
}

// setupTenderHandlers configures handlers for tender scraping phase
func (cs *CorrScraper) setupTenderHandlers() {
	cs.collector.OnHTML("table.list_table", cs.handleTenderTable)
	cs.collector.OnHTML("a#loadNext", cs.handleNextButton)
	cs.collector.OnHTML("span:contains('Total records:')", cs.handleTotalRecords)
	cs.collector.OnError(cs.handleError)
}

// clearHandlers removes all existing handlers
func (cs *CorrScraper) clearHandlers() {
	oldCollector := cs.collector
	cs.collector = cs.collector.Clone()

	// Copy cookies from old collector for baseURL
	cookies := oldCollector.Cookies(cs.baseURL)
	if len(cookies) > 0 {
		cs.collector.SetCookies(cs.baseURL, cookies)
	}
}

// handleError handles errors during scraping
func (cs *CorrScraper) handleError(r *colly.Response, err error) {
	cs.logger.Printf("ERROR: Request failed - URL: %s, Status: %d, Error: %v",
		r.Request.URL, r.StatusCode, err)
}

// handleTenderTable processes the tender results table (Step 3)
func (cs *CorrScraper) handleTenderTable(e *colly.HTMLElement) {
	// Debug File, save the html content to file
	filename := fmt.Sprintf("Corrigendum_Page_%d.html", cs.currentPage)
	if err := cs.saveFile("TenderData/Pages", filename, e.Response.Body); err != nil {
		cs.logger.Printf("ERROR: Failed to save page HTML: %v", err)
	}

	cs.logger.Printf("PAGE %d: Found tender table! Parsing results... (Status: %d)", cs.currentPage, e.Response.StatusCode)
	cs.resultsFound = true
	cs.sessionEstablished = true

	if err := cs.parseTenders(e); err != nil {
		cs.logger.Printf("ERROR: %v", err)
		if cs.failedWriter != nil {
			cs.failedWriter.WriteFailure(cs.state, err.Error()) // Only log the website failure
		}
	}
}

// Test it for a small number of tenders to see it stops after scraping whole page
func (cs *CorrScraper) handleNextButton(e *colly.HTMLElement) {
	href := e.Attr("href")
	if href == "" {
		cs.nextButtonURL = ""
		cs.logger.Printf("PAGE %d: No more Next button found - reached last page", cs.currentPage)
		return
	}

	base, err := url.Parse(e.Request.URL.String())
	if err != nil {
		cs.logger.Printf("ERROR: Failed to parse base URL: %v", err)
		cs.nextButtonURL = href
		return
	}

	rel, err := url.Parse(href)
	if err != nil {
		cs.logger.Printf("ERROR: Failed to parse href: %v", err)
		cs.nextButtonURL = href
		return
	}

	cs.nextButtonURL = base.ResolveReference(rel).String()
	cs.logger.Printf("PAGE %d: Next button found: %s", cs.currentPage, cs.nextButtonURL)
}

// handleTotalRecords extracts the total records count
func (cs *CorrScraper) handleTotalRecords(e *colly.HTMLElement) {
	text := e.Text
	// Extract number from "Total records: 8174"
	re := regexp.MustCompile(`Total records:\s*(\d+)`)
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		if total, err := strconv.Atoi(matches[1]); err == nil {
			if cs.totalTenders == 0 { // Only set on first page
				cs.totalTenders = total
				cs.logger.Printf("Total records found: %d", cs.totalTenders)
			}
		}
	}
}

// parseTenders parses tender data from HTML element
// Returns an error if parsing fails for the page (website-level failure)
func (cs *CorrScraper) parseTenders(e *colly.HTMLElement) error {
	cs.logger.Printf("PAGE %d: Parsing tender data from table element...", cs.currentPage)
	var tendersFoundOnPage int

	// Track if any row parsing fails (non-fatal)
	var parseFailed bool

	e.DOM.Find("tr.odd, tr.even, tr[id^=informal]").Each(func(i int, s *goquery.Selection) {
		tendersFoundOnPage++
		cs.scrapedTenders++

		cells := s.Find("td")
		if cells.Length() < 6 {
			cs.logger.Printf("WARNING: Row %d has less than 6 columns, skipping", i)
			parseFailed = true
			return
		}

		closingDate := strings.TrimSpace(cells.Eq(2).Text())
		ePublishedDate := strings.TrimSpace(cells.Eq(1).Text())
		linkTag := cells.Eq(4).Find("a")
		title := strings.TrimSpace(linkTag.Text())
		href, _ := linkTag.Attr("href")
		organisation := strings.TrimSpace(cells.Eq(5).Text())

		// Extract tender ID
		cellText := strings.TrimSpace(cells.Eq(4).Text())
		// cellText = utils.CleanField(cellText)
		tenderID := utils.ExtractTenderID(cellText)

		// Build full link
		fullLink := href
		if href != "" {
			if base, err := url.Parse(cs.baseURL); err == nil {
				if rel, err := url.Parse(href); err == nil {
					fullLink = base.ResolveReference(rel).String()
				}
			} else {
				parseFailed = true
			}
		}

		// Write row to CSV
		if err := cs.csvWriter.Write([]string{
			strconv.Itoa(cs.scrapedTenders),
			tenderID,
			strconv.Itoa(cs.currentPage),
			title,
			ePublishedDate,
			organisation,
			closingDate,
			fullLink,
			cellText,
		}); err != nil {
			cs.logger.Printf("ERROR: Failed to write CSV row: %v", err)
			parseFailed = true
		}
	})

	cs.csvWriter.Flush()
	cs.logger.Printf("PAGE %d: Parsed %d tenders", cs.currentPage, tendersFoundOnPage)

	if parseFailed {
		return fmt.Errorf("failed to parse page %d", cs.currentPage)
	}

	if tendersFoundOnPage == 0 {
		cs.logger.Printf("PAGE %d: No tenders found (likely last page or empty dataset)", cs.currentPage)
		return nil
	}

	return nil
}

// saveFile utility function
func (cs *CorrScraper) saveFile(dir, filename string, body []byte) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("could not create directory %s: %w", dir, err)
	}

	fullPath := filepath.Join(dir, filename)
	if err := os.WriteFile(fullPath, body, 0644); err != nil {
		return fmt.Errorf("could not write file %s: %w", fullPath, err)
	}

	cs.logger.Printf("Saved: %s", fullPath)
	return nil
}
