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
	"github.com/vx6fid/tender-scraper/scraper/captcha"
)

// TenderScraper holds the state and configuration for the scraping process.
type TenderScraper struct {
	collector          *colly.Collector
	baseURL            string
	captchaSolved      bool
	sessionEstablished bool
	resultsFound       bool
	activeTendersURL   string
	totalTenders       int
	scrapedTenders     int
	currentPage        int
	nextButtonURL      string
	csvFile            *os.File
	csvWriter          *csv.Writer
}

// NewTenderScraper initializes a new scraper instance.
func NewTenderScraper(c *colly.Collector, baseURL string) *TenderScraper {
	c.AllowURLRevisit = true

	return &TenderScraper{
		collector:        c,
		baseURL:          baseURL,
		activeTendersURL: baseURL + "?page=FrontEndLatestActiveTenders&service=page",
		currentPage:      1,
	}
}

// Main entry point to start the scraping process
func (ts *TenderScraper) ScrapeActiveTenders() error {
	log.Println("Starting tender scraping process with correct session flow.")

	// open CSV
	file, err := os.Create("tenders.csv")
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

	// STEP 1: Click on Active Tenders link to get captcha form
	log.Printf("STEP 1: Clicking Active Tenders link: %s", ts.activeTendersURL)
	ts.setupCaptchaHandlers()

	if err := ts.collector.Visit(ts.activeTendersURL); err != nil {
		return fmt.Errorf("failed to visit active tenders page for captcha: %w", err)
	}

	if !ts.captchaSolved {
		return fmt.Errorf("failed to solve captcha in step 1")
	}

	// STEP 2: Wait for session to be established
	log.Println("STEP 2: Captcha solved, waiting for session establishment...")
	time.Sleep(1 * time.Second)

	// STEP 3: Click Active Tenders link AGAIN with established session
	log.Printf("STEP 3: Clicking Active Tenders link again with session: %s", ts.activeTendersURL)

	// Clear handlers and setup for tender parsing
	ts.clearHandlers()
	ts.setupTenderHandlers()

	if err := ts.collector.Visit(ts.activeTendersURL); err != nil {
		return fmt.Errorf("failed to visit active tenders page with session: %w", err)
	}

	if !ts.resultsFound {
		return fmt.Errorf("no tender results found after establishing session")
	}

	// STEP 4: Handle pagination - keep clicking "Next" until no more pages
	log.Println("STEP 4: Starting pagination process...")
	if err := ts.handlePagination(); err != nil {
		return fmt.Errorf("pagination failed: %w", err)
	}

	log.Printf("Scraping completed successfully! Total tenders scraped: %d/%d", ts.scrapedTenders, ts.totalTenders)
	return nil
}

// handlePagination processes all pages by clicking Next button
func (ts *TenderScraper) handlePagination() error {
	for ts.nextButtonURL != "" {
		// stop if scraped enough tenders
		if ts.totalTenders > 0 && ts.scrapedTenders >= ts.totalTenders {
			log.Printf("All %d tenders scraped, stopping pagination", ts.scrapedTenders)
			break
		}

		log.Printf("PAGE %d: Clicking Next button: %s", ts.currentPage+1, ts.nextButtonURL)

		// Small delay between page requests
		time.Sleep(500 * time.Millisecond)

		// Visit the next page
		if err := ts.collector.Visit(ts.nextButtonURL); err != nil {
			log.Printf("ERROR: Failed to visit next page: %v", err)
			break
		}

		ts.currentPage++

		// Log progress
		if ts.totalTenders > 0 {
			progress := float64(ts.scrapedTenders) / float64(ts.totalTenders) * 100
			log.Printf("Progress: %d/%d tenders (%.1f%%) - Page %d completed",
				ts.scrapedTenders, ts.totalTenders, progress, ts.currentPage)
		}
	}

	log.Printf("Pagination completed! Processed %d pages, scraped %d tenders", ts.currentPage, ts.scrapedTenders)
	return nil
}

// setupCaptchaHandlers configures handlers for captcha solving phase
func (ts *TenderScraper) setupCaptchaHandlers() {
	ts.collector.OnHTML("form#LatestActiveTenders", ts.handleCaptchaForm)
	ts.collector.OnResponse(ts.handleCaptchaResponse)
	ts.collector.OnError(ts.handleError)
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
	log.Printf("ERROR: Request failed - URL: %s, Status: %d, Error: %v",
		r.Request.URL, r.StatusCode, err)
}

// handleCaptchaForm processes the initial captcha form (Step 1)
func (ts *TenderScraper) handleCaptchaForm(e *colly.HTMLElement) {
	// ts.saveFile("debug", "Before Calling Captcha", e.Response.Body)
	if ts.captchaSolved {
		return
	}

	log.Printf("STEP 1: Found captcha form, processing... (Status: %d)", e.Response.StatusCode)

	// Find captcha image
	captchaImgSrc, _ := e.DOM.Find("img#captchaImage").Attr("src")
	if captchaImgSrc == "" {
		captchaImgSrc = e.DOM.Find("img[id*='captcha'], img[src*='captcha']").AttrOr("src", "")
	}
	if captchaImgSrc == "" {
		captchaImgSrc = e.DOM.Parent().Find("img#captchaImage, img[id*='captcha']").AttrOr("src", "")
	}

	if captchaImgSrc == "" {
		log.Println("ERROR: Could not find captcha image")
		return
	}

	log.Println("Captcha image found, solving...")

	// Solve captcha
	captchaSolution, err := captcha.ManualCaptchaSolver(captchaImgSrc)
	if err != nil {
		log.Printf("ERROR: Captcha solving failed: %v", err)
		return
	}

	log.Printf("Captcha solved: %s", captchaSolution)
	ts.submitCaptchaForm(e, captchaSolution)
}

// submitCaptchaForm submits the captcha to establish session
func (ts *TenderScraper) submitCaptchaForm(e *colly.HTMLElement, captchaSolution string) {
	// Save the full response body for debugging
	// ts.saveFile("debug", "CaptchaForm.html", e.Response.Body)

	// Parse the entire response body to find all form fields
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(e.Response.Body)))
	if err != nil {
		log.Printf("ERROR: Failed to parse response body: %v", err)
		return
	}

	formData := make(map[string]string)
	log.Println("Collecting form fields from LatestActiveTenders form:")

	// Find inputs specifically within the LatestActiveTenders form
	doc.Find("form#LatestActiveTenders").Each(func(_ int, form *goquery.Selection) {
		form.Find("input[name]").Each(func(_ int, s *goquery.Selection) {
			name, _ := s.Attr("name")
			value, _ := s.Attr("value")
			inputType, _ := s.Attr("type")

			if name != "" {
				formData[name] = value
				log.Printf("  [%s] %s = %s", inputType, name, value)
			}
		})
	})

	// If that didn't work, try a more direct approach
	if len(formData) < 5 {
		log.Println("Form-specific search failed, trying broader search...")

		doc.Find("input[name]").Each(func(_ int, s *goquery.Selection) {
			name, _ := s.Attr("name")
			value, _ := s.Attr("value")
			inputType, _ := s.Attr("type")

			if name != "" && name != "domainUrl" {
				if s.Closest("form#WebBorder_0").Length() == 0 {
					formData[name] = value
					log.Printf("  [BROAD-%s] %s = %s", inputType, name, value)
				}
			}
		})
	}

	// Override/add the required fields for captcha submission
	formData["captchaText"] = captchaSolution
	formData["Submit"] = "Search"

	log.Printf("Submitting captcha form to: %s", ts.baseURL)
	log.Printf("Form data has %d fields total", len(formData))

	// Log all form data being submitted
	log.Println("Final form data being submitted:")
	for key, value := range formData {
		if len(value) > 50 {
			log.Printf("  %s = %s... [truncated]", key, value[:50])
		} else {
			log.Printf("  %s = %s", key, value)
		}
	}

	if err := e.Request.Post(ts.baseURL, formData); err != nil {
		log.Printf("ERROR: Failed to submit captcha form: %v", err)
	}
}

// handleCaptchaResponse processes response after captcha submission
func (ts *TenderScraper) handleCaptchaResponse(r *colly.Response) {
	log.Printf("STEP 1: Processing captcha response - URL: %s, Method: %s, Status: %d",
		r.Request.URL, r.Request.Method, r.StatusCode)

	if ts.captchaSolved || r.Request.Method != "POST" {
		return
	}

	bodyStr := string(r.Body)

	// Check if captcha was successful
	hasError := strings.Contains(bodyStr, "Invalid input request") ||
		strings.Contains(bodyStr, "incorrect") ||
		strings.Contains(bodyStr, "wrong")

	stillHasCaptcha := strings.Contains(bodyStr, "captchaImage") ||
		strings.Contains(bodyStr, "captchaText")

	if hasError {
		log.Printf("ERROR: Captcha was rejected by server (Status: %d)", r.StatusCode)
		// _ = ts.saveFile("debug", "step1_captcha_rejected.html", r.Body)
		return
	}

	if stillHasCaptcha {
		log.Printf("WARNING: Server still showing captcha - may have been incorrect (Status: %d)", r.StatusCode)
		// _ = ts.saveFile("debug", "step1_captcha_still_showing.html", r.Body)
		return
	}

	// If we reach here, captcha was likely successful
	log.Printf("SUCCESS: Captcha appears to have been accepted! (Status: %d)", r.StatusCode)
	log.Println("Session should now be established with cookies")

	// Log cookies for debugging
	if cookies := ts.collector.Cookies(r.Request.URL.String()); len(cookies) > 0 {
		log.Printf("Session cookies received: %d cookies", len(cookies))
		for _, cookie := range cookies {
			log.Printf("  Cookie: %s = %s", cookie.Name, cookie.Value)
		}
	} else {
		log.Println("WARNING: No cookies received after captcha submission")
	}

	ts.captchaSolved = true
	// _ = ts.saveFile("results", "step1_captcha_success.html", r.Body)
}

// handleTenderTable processes the tender results table (Step 3)
func (ts *TenderScraper) handleTenderTable(e *colly.HTMLElement) {
	log.Printf("PAGE %d: Found tender table! Parsing results... (Status: %d)", ts.currentPage, e.Response.StatusCode)
	ts.resultsFound = true
	ts.sessionEstablished = true
	ts.parseTenders(e)
}

// Test it for a small number of tenders to see it stops after scraping whole page
func (ts *TenderScraper) handleNextButton(e *colly.HTMLElement) {
	href := e.Attr("href")
	if href == "" {
		ts.nextButtonURL = ""
		log.Printf("PAGE %d: No more Next button found - reached last page", ts.currentPage)
		return
	}

	base, err := url.Parse(e.Request.URL.String())
	if err != nil {
		log.Printf("ERROR: Failed to parse base URL: %v", err)
		ts.nextButtonURL = href
		return
	}

	rel, err := url.Parse(href)
	if err != nil {
		log.Printf("ERROR: Failed to parse href: %v", err)
		ts.nextButtonURL = href
		return
	}

	ts.nextButtonURL = base.ResolveReference(rel).String()
	log.Printf("PAGE %d: Next button found: %s", ts.currentPage, ts.nextButtonURL)
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
				log.Printf("Total records found: %d", ts.totalTenders)
			}
		}
	}
}

// parseTenders parses tender data from HTML element
func (ts *TenderScraper) parseTenders(e *colly.HTMLElement) {
	log.Printf("PAGE %d: Parsing tender data from table element...", ts.currentPage)
	var tendersFoundOnPage int
	// ts.saveFile("debug", fmt.Sprintf("Page_%d.html", ts.currentPage), []byte(e.Response.Body))

	e.DOM.Find("tr.even, tr.odd").Each(func(i int, s *goquery.Selection) {
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
				log.Printf("ERROR: Failed to write CSV row: %v", err)
			}

			log.Printf("  Tender %d (Page %d): '%s' ", ts.scrapedTenders, ts.currentPage, title)
		}

	})

	ts.csvWriter.Flush()

	log.Printf("PAGE %d: Successfully parsed %d tenders", ts.currentPage, tendersFoundOnPage)
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

	log.Printf("Saved: %s", fullPath)
	return nil
}
