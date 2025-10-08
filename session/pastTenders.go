package session

import (
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/vx6fid/tender-scraper/scraper/captcha"
)

// EstablishTenderStatusSession performs the tender status captcha flow and waits for session establishment
func (s *Session) EstablishTenderStatusSession(state, tenderStatus, fromDate, toDate string) error {
	startTime := time.Now()
	host := HostFromURL(s.BaseURL)
	s.captchaCollector = s.NewCollector(host) // This automatically shares the cookie jar

	// Store search parameters for use in form submission
	searchParams := map[string]string{
		"tenderStatus": tenderStatus,
		"fromDate":     fromDate,
		"toDate":       toDate,
	}

	// Set up handlers
	s.captchaCollector.OnHTML("form#frmSearchFilter", func(e *colly.HTMLElement) {
		s.handleTenderStatusForm(e, searchParams)
	})

	s.captchaCollector.OnResponse(func(r *colly.Response) {
		s.handleTenderStatusResponse(r)
	})

	s.captchaCollector.OnError(func(r *colly.Response, err error) {
		s.handleError(r, err)
	})

	// Start the flow
	s.logger.Printf("STEP 1: Starting tender status captcha/session flow: %s", s.ResultsURL)
	if err := s.captchaCollector.Visit(s.ResultsURL); err != nil {
		return fmt.Errorf("failed to visit tender status page for captcha: %w", err)
	}

	// Wait for session establishment (same pattern as original EstablishSession)
	timeout := time.After(60 * time.Second) // Slightly longer timeout for captcha solving
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for tender status session establishment")
		case <-tick.C:
			if s.sessionEstablished {
				if err := s.validateTenderStatusSession(); err != nil {
					return err
				}
				log.Printf("[%s] Tender status session established successfully in %s", state, time.Since(startTime))
				s.logger.Printf("Tender status session established successfully")
				return nil
			}
		}
	}

}

// handleTenderStatusForm handles the tender status search form with captcha
func (s *Session) handleTenderStatusForm(e *colly.HTMLElement, searchParams map[string]string) {
	s.logger.Printf("[tender-status] found form at %s", e.Request.URL.String())

	// Check if results table already exists (session might already be valid)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(e.Response.Body)))
	if err != nil {
		s.logger.Printf("[tender-status] parse error: %v", err)
		return
	}

	// Check for results table first
	if s.hasResultsTable(doc) {
		s.logger.Println("[tender-status] results table found, session already established")
		s.sessionEstablished = true
		return
	}

	// Need to solve captcha and submit form
	s.logger.Printf("[tender-status] no results table found, proceeding with captcha")

	// Extract form data
	formData, err := s.extractTenderStatusFormData(doc)
	if err != nil {
		s.logger.Printf("[tender-status] form data extraction failed: %v", err)
		return
	}

	// Override with search parameters
	for key, value := range searchParams {
		formData[key] = value
	}
	formData["Search"] = "Search"

	// Find and solve captcha
	captchaSrc := s.findCaptchaImageSrc(e)
	if captchaSrc == "" {
		s.logger.Println("[tender-status] no captcha image found")
		return
	}

	s.logger.Printf("[tender-status] solving captcha...")
	solution, err := captcha.LocalCaptchaSolver(captchaSrc, s.logger)
	if err != nil {
		s.logger.Printf("[tender-status] captcha solver error: %v", err)
		return
	}

	s.logger.Printf("[tender-status] captcha solved: %s", solution)
	formData["captchaText"] = solution

	// Submit form
	s.logger.Printf("[tender-status] submitting form with %d fields", len(formData))
	if err := e.Request.Post(s.BaseURL, formData); err != nil {
		s.logger.Printf("[tender-status] form submission failed: %v", err)
	}
}

// extractTenderStatusFormData extracts all form fields from the tender status form
func (s *Session) extractTenderStatusFormData(doc *goquery.Document) (map[string]string, error) {
	formData := make(map[string]string)

	// Extract all input fields within the search form
	doc.Find("form#frmSearchFilter input[name]").Each(func(_ int, input *goquery.Selection) {
		if name, exists := input.Attr("name"); exists && name != "" {
			value, _ := input.Attr("value")
			formData[name] = value
		}
	})

	// Extract select fields
	doc.Find("form#frmSearchFilter select[name]").Each(func(_ int, sel *goquery.Selection) {
		if name, exists := sel.Attr("name"); exists && name != "" {
			// Get selected option or first option as fallback
			value := sel.Find("option[selected]").AttrOr("value", "")
			if value == "" {
				value = sel.Find("option").First().AttrOr("value", "")
			}
			formData[name] = value
		}
	})

	// Fallback: if we didn't get enough fields, try broader search
	if len(formData) < 3 {
		s.logger.Printf("[tender-status] form had few fields (%d), trying broader search", len(formData))
		doc.Find("input[name]").Each(func(_ int, input *goquery.Selection) {
			if name, exists := input.Attr("name"); exists && name != "" {
				// Skip inputs from irrelevant forms
				if input.Closest("form#WebBorder_0").Length() == 0 {
					value, _ := input.Attr("value")
					formData[name] = value
				}
			}
		})
	}

	s.logger.Printf("[tender-status] extracted %d form fields", len(formData))
	return formData, nil
}

// findCaptchaImageSrc finds the captcha image source using multiple selectors
func (s *Session) findCaptchaImageSrc(e *colly.HTMLElement) string {
	selectors := []string{
		"img#captchaImage",
		"img[id*='captcha']",
		"img[src*='captcha']",
		"img[alt*='captcha']",
		"img[class*='captcha']",
	}

	// Try direct selectors
	for _, selector := range selectors {
		if src := e.DOM.Find(selector).AttrOr("src", ""); src != "" {
			s.logger.Printf("[tender-status] found captcha: %s", src)
			return src
		}
	}

	// Try parent element
	for _, selector := range selectors {
		if src := e.DOM.Parent().Find(selector).AttrOr("src", ""); src != "" {
			s.logger.Printf("[tender-status] found captcha in parent: %s", src)
			return src
		}
	}

	return ""
}

// handleTenderStatusResponse handles POST responses from tender status form submission
func (s *Session) handleTenderStatusResponse(r *colly.Response) {
	s.logger.Printf("[tender-status] response: %s | %s | %d", r.Request.Method, r.Request.URL.String(), r.StatusCode)

	if r.Request.Method != "POST" {
		return // Only care about POST responses
	}

	body := string(r.Body)

	// Save HTML response to file for debugging
	// filename := fmt.Sprintf("tender_status_response_%s.html", time.Now().Format("20060102_150405"))
	// if err := os.WriteFile(filename, r.Body, 0644); err != nil {
	// 	s.logger.Printf("[tender-status] failed to save response HTML: %v", err)
	// } else {
	// 	s.logger.Printf("[tender-status] saved response HTML to: %s", filename)
	// }

	// Check for obvious errors
	hasError := strings.Contains(strings.ToLower(body), "invalid input request") ||
		strings.Contains(strings.ToLower(body), "incorrect") ||
		strings.Contains(strings.ToLower(body), "wrong") ||
		strings.Contains(strings.ToLower(body), "invalid captcha")

	if hasError {
		s.logger.Printf("[tender-status] server rejected submission")
		return
	}

	// The key insight: we need to verify success by checking if results table appears
	// We'll do this by visiting the ResultsURL again after POST
	s.verifyTenderStatusSession()
}

// verifyTenderStatusSession verifies the session by visiting ResultsURL and checking for table
func (s *Session) verifyTenderStatusSession() {
	s.logger.Printf("[tender-status] verifying session by checking results table...")

	// Create a verification collector that shares the same session
	verifyCollector := s.NewCollector(HostFromURL(s.BaseURL))

	verifyCollector.OnResponse(func(r *colly.Response) {
		s.logger.Printf("[tender-status-verify] response: %d bytes", len(r.Body))

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(r.Body)))
		if err != nil {
			s.logger.Printf("[tender-status-verify] parse error: %v", err)
			return
		}

		if s.hasResultsTable(doc) {
			s.logger.Printf("[tender-status-verify] SUCCESS: Results table found!")

			// Check cookies
			u, _ := url.Parse(s.BaseURL)
			if cookies := s.Jar.Cookies(u); len(cookies) > 0 {
				s.logger.Printf("[tender-status-verify] session cookies: %d", len(cookies))
				s.sessionEstablished = true
			} else {
				s.logger.Printf("[tender-status-verify] no cookies found despite table presence")
			}
		} else {
			s.logger.Printf("[tender-status-verify] no results table found")
		}
	})

	// Visit ResultsURL to check for table
	if err := verifyCollector.Visit(s.ResultsURL); err != nil {
		s.logger.Printf("[tender-status-verify] verification visit failed: %v", err)
	}
}

func (s *Session) hasResultsTable(doc *goquery.Document) bool {
	// Check for various table selectors
	tableSelectors := []string{
		"table#tabList",
	}

	for _, selector := range tableSelectors {
		table := doc.Find(selector)
		if table.Length() > 0 {
			s.logger.Printf("[tender-status] found results table: %s", selector)

			// Print first row for debugging
			// firstRow := table.Find("tr").First()
			// if firstRow.Length() > 0 {
			// 	rowText := strings.TrimSpace(firstRow.Text())
			// 	s.logger.Printf("[tender-status] first row: %s", rowText)
			// }

			return true
		}
	}

	// Fallback: check for any substantial table (more than just headers)
	tables := doc.Find("table")
	foundDataTable := false
	tables.Each(func(_ int, table *goquery.Selection) {
		if foundDataTable {
			return
		}
		rows := table.Find("tr")
		// Check if the rows have id starting with "informal"
		if rows.Length() > 2 && strings.HasPrefix(rows.Eq(1).AttrOr("id", ""), "informal") {
			s.logger.Printf("[tender-status] found data table with %d rows", rows.Length())
			foundDataTable = true
		}
	})

	return foundDataTable
}

// validateTenderStatusSession performs final validation of the established session
func (s *Session) validateTenderStatusSession() error {
	u, _ := url.Parse(s.BaseURL)
	cookies := s.Jar.Cookies(u)

	if len(cookies) == 0 {
		return fmt.Errorf("no cookies found after session establishment")
	}

	s.logger.Printf("Tender status session established with %d cookies:", len(cookies))
	for _, cookie := range cookies {
		s.logger.Printf("  cookie: %s = %s", cookie.Name, cookie.Value)
	}

	return nil
}
