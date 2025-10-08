package nav

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/vx6fid/tender-scraper/session"
)

// PastScraper holds the state and configuration for the scraping process.
type PastScraper struct {
	collector      *colly.Collector
	baseURL        string
	state          string
	ResultsURL     string
	totalTenders   int
	scrapedTenders int
	currentPage    int
	nextButtonURL  string
	fromDate       string
	toDate         string
	logger         *log.Logger
	pastWriter     *PastWriter
	failedWriter   *FailedWriter
}

func NewPastScraper(sess *session.Session, domain string, state string, headers []string, writer *PastWriter, failedWriter *FailedWriter) (*PastScraper, error) {
	collector := sess.NewCollector(domain)
	collector.AllowURLRevisit = true

	// If no writer was passed, create one (fallback)
	var pastWriter *PastWriter
	pastWriter = writer

	return &PastScraper{
		collector:  collector,
		baseURL:    sess.BaseURL,
		state:      state,
		ResultsURL: sess.ResultsURL,
		// fromDate:       fromDate,
		// toDate:         toDate,
		scrapedTenders: 0,
		currentPage:    1,
		nextButtonURL:  sess.BaseURL + "?component=loadNext&page=WebTenderStatusLists&service=direct&session=T",
		logger:         log.New(os.Stdout, "PastScraper: ", log.LstdFlags),
		pastWriter:     pastWriter,
		failedWriter:   failedWriter,
	}, nil
}

// WriteRow writes a row to the CSV file
func (ps *PastScraper) WriteRow(row []string) {
	ps.pastWriter.WriteRow(row)
}

// Close closes the CSV writer and waits for all writes to complete
func (ps *PastScraper) Close() {
	if ps.pastWriter != nil {
		ps.pastWriter.Close()
		ps.logger.Printf("CSV writer closed for state: %s", ps.state)
	}
}

func (ps *PastScraper) Run() error {
	// ps.logger.Printf("Starting scraping for state: %s", ps.state)

	// Clear handlers and setup for tender parsing
	ps.setupHandlers()

	// Start scraping from the initial page
	err := ps.collector.Visit(ps.ResultsURL)
	if err != nil {
		if ps.failedWriter != nil {
			ps.failedWriter.WriteFailure(ps.fromDate, ps.toDate, err.Error())
		}
		return fmt.Errorf("failed to visit initial page: %w", err)
	}

	// ps.logger.Println("Starting pagination process...")
	if err := ps.handlePagination(); err != nil {
		return fmt.Errorf("pagination failed: %w", err)
	}

	// ps.logger.Printf("Scraping completed successfully! Total tenders scraped: %d/%d", ps.scrapedTenders, ps.totalTenders)
	return nil
}

func (ps *PastScraper) setupHandlers() {
	ps.collector.OnHTML("table#tabList", ps.handleTenderTable)
	ps.collector.OnHTML("span:contains('Total records:')", ps.handleTotalRecords)
	ps.collector.OnError(ps.handleError)
}

// handleError handles errors during scraping
func (ps *PastScraper) handleError(r *colly.Response, err error) {
	ps.logger.Printf("ERROR: Request failed - URL: %s, Status: %d, Error: %v",
		r.Request.URL, r.StatusCode, err)
}

// handleTenderTable processes the tender results table (Step 3)
func (ps *PastScraper) handleTenderTable(e *colly.HTMLElement) {
	// ps.logger.Printf("PAGE %d: Found tender table! Parsing results... (Status: %d)", ps.currentPage, e.Response.StatusCode)
	ps.parseTenders(e)
}

// handleTotalRecords extracts the total records count
func (ps *PastScraper) handleTotalRecords(e *colly.HTMLElement) {
	text := e.Text
	// Extract number from "Total records: 8174"
	re := regexp.MustCompile(`Total records:\s*(\d+)`)
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		if total, err := strconv.Atoi(matches[1]); err == nil {
			if ps.totalTenders == 0 { // Only set on first page
				ps.totalTenders = total
				// ps.logger.Printf("Total records found: %d", ps.totalTenders)
			}
		}
	}
}

// handlePagination processes all pages by clicking Next button
// func (ps *PastScraper) handlePagination() error {
// 	for ps.nextButtonURL != "" {
// 		// stop if scraped enough tenders
// 		if ps.totalTenders > 0 && ps.scrapedTenders >= ps.totalTenders {
// 			ps.logger.Printf("All %d tenders scraped, stopping pagination", ps.scrapedTenders)
// 			break
// 		}

// 		ps.logger.Printf("PAGE %d: Clicking Next button: %s", ps.currentPage+1, ps.nextButtonURL)

// 		// Small delay between page requests
// 		time.Sleep(500 * time.Millisecond)

// 		// Visit the next page
// 		if err := ps.collector.Visit(ps.nextButtonURL); err != nil {
// 			ps.logger.Printf("failed to visit page: %v", err)
// 			if ps.failedWriter != nil {
// 				ps.failedWriter.WriteFailure(ps.fromDate, ps.toDate, err.Error())
// 			}
// 			break
// 		}

// 		if ps.scrapedTenders == 0 {
// 			ps.logger.Printf("No tenders found on page %d", ps.currentPage)
// 			break
// 		}

// 		ps.currentPage++

// 		// Log progress
// 		if ps.totalTenders > 0 {
// 			progress := float64(ps.scrapedTenders) / float64(ps.totalTenders) * 100
// 			ps.logger.Printf("Progress: %d/%d tenders (%.1f%%) - Page %d completed",
// 				ps.scrapedTenders, ps.totalTenders, progress, ps.currentPage)
// 		}
// 	}

// 	ps.logger.Printf("Pagination completed! Processed %d pages, scraped %d tenders", ps.currentPage, ps.scrapedTenders)
// 	return nil
// }

// handlePagination
func (ps *PastScraper) handlePagination() error {
	for ps.nextButtonURL != "" {
		// stop if scraped enough tenders
		if ps.totalTenders > 0 && ps.scrapedTenders >= ps.totalTenders {
			// ps.logger.Printf("All %d tenders scraped, stopping pagination", ps.scrapedTenders)
			break
		}

		// ps.logger.Printf("PAGE %d: Clicking Next button: %s", ps.currentPage+1, ps.nextButtonURL)
		time.Sleep(500 * time.Millisecond)

		prevScraped := ps.scrapedTenders
		if err := ps.collector.Visit(ps.nextButtonURL); err != nil {
			ps.logger.Printf("failed to visit page: %v", err)
			break
		}

		// if tender count didn’t increase, stop
		if ps.scrapedTenders == prevScraped {
			// ps.logger.Printf("No new tenders found after page %d, stopping.", ps.currentPage)
			break
		}

		ps.currentPage++
	}

	// ps.logger.Printf("Pagination completed! Processed %d pages, scraped %d tenders", ps.currentPage, ps.scrapedTenders)
	return nil
}

// parseTenders parses tender data from HTML element
func (ps *PastScraper) parseTenders(e *colly.HTMLElement) {
	// ps.logger.Printf("PAGE %d: Parsing tender data from table element...", ps.currentPage)
	var tendersFoundOnPage int
	// ps.saveFile("debug", fmt.Sprintf("Page_%d.html", ps.currentPage), []byte(e.Response.Body))

	// e.DOM.Find("tr.even, tr.odd").Each(func(i int, s *goquery.Selection) {
	e.DOM.Find(`tr[id^="informal"]`).Each(func(i int, s *goquery.Selection) {
		tendersFoundOnPage++
		ps.scrapedTenders++
		cells := s.Find("td")

		if cells.Length() >= 6 {
			linkTag := cells.Eq(5).Find("a")
			href, exists := linkTag.Attr("href")
			if !exists {
				href = "" // no link present
			}

			tenderID := cells.Eq(1).Text()

			titleParts := strings.Split(cells.Eq(2).Text(), "][")
			title := titleParts[0]

			organisation := strings.TrimSpace(cells.Eq(3).Text())
			tenderStage := cells.Eq(4).Text()
			// remove the [ from the start
			title = strings.TrimPrefix(title, "[")

			fullLink := href
			if href != "" {
				base, err := url.Parse(ps.baseURL)
				if err == nil {
					rel, err := url.Parse(href)
					if err == nil {
						fullLink = base.ResolveReference(rel).String()
					}
				}
			}

			// Print tender details to screen
			// ps.logger.Printf("  Tender %d (Page %d): '%s' \n", ps.scrapedTenders, ps.currentPage, title)
			// ps.logger.Printf("    ID: %s\n", tenderID)
			// ps.logger.Printf("    Organisation: %s\n", organisation)
			// ps.logger.Printf("    Stage: %s\n", tenderStage)
			// ps.logger.Printf("    Link: %s\n", fullLink)
			// ps.logger.Println("-----------------------")

			ps.WriteRow([]string{
				strconv.Itoa(ps.scrapedTenders),
				tenderID,
				strconv.Itoa(ps.currentPage),
				title,
				organisation,
				tenderStage,
				fullLink,
			})

		}

	})

	// ps.logger.Printf("PAGE %d: Successfully parsed %d tenders", ps.currentPage, tendersFoundOnPage)
}
