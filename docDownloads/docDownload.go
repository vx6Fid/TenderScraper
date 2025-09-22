package docdownload

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/vx6fid/tender-scraper/session"
)

// DocDownloader handles NIT and Zip downloads using a single collector
type DocDownloader struct {
	sess        *session.Session
	state       string
	logger      *log.Logger
	collector   *colly.Collector
	NITDocs     []NITDocument
	WorkItemZip WorkItemDocument
}

// Config holds configurable parameters
type Config struct {
	CaptchaTimeout time.Duration
	PollInterval   time.Duration
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		CaptchaTimeout: 40 * time.Second,
		PollInterval:   500 * time.Millisecond,
	}
}

// NewDocDownloader creates a new downloader with a single collector
func NewDocDownloader(sess *session.Session, state string, logger *log.Logger) *DocDownloader {
	collector := sess.NewCollector(session.HostFromURL(sess.BaseURL))

	return &DocDownloader{
		sess:      sess,
		state:     state,
		logger:    logger,
		collector: collector,
	}
}

// RunTender downloads documents for a given tender URL
// Assumes the doc captcha has already been solved (or solves it if not)
func (d *DocDownloader) Run(tenderURL string) error {
	d.logger.Printf("[%s][docDownload] solving doc captcha", d.state)
	if err := d.SolveDocCaptcha(); err != nil {
		return err
	}

	d.logger.Printf("[%s][docDownload] visiting tender page: %s", d.state, tenderURL)

	// Extract document links
	documentLinks := d.extractDocumentLinks(tenderURL)
	if len(documentLinks) == 0 {
		d.logger.Printf("[%s][docDownload] no document links found", d.state)
		return nil
	}

	// Must have DirectLinks now
	if !d.hasDirectLinks(documentLinks) {
		return fmt.Errorf("[%s][docDownload] unexpected: no DirectLinks found even after captcha", d.state)
	}

	d.processDirectLinks(documentLinks)

	// Download files locally
	if err := d.downloadFiles(); err != nil {
		return err
	}

	// Upload to AWS under tenderID "dummyID"
	if err := d.processAndUploadDocs("dummyID", "tenderbharat"); err != nil {
		return err
	}

	return nil
}

// clearHandlers resets the collector while keeping the session cookies
func (d *DocDownloader) clearHandlers() {
	oldCollector := d.collector
	d.collector = oldCollector.Clone() // creates a fresh collector

	// Copy cookies from old collector to new one
	cookies := oldCollector.Cookies(d.sess.BaseURL)
	if len(cookies) > 0 {
		d.collector.SetCookies(d.sess.BaseURL, cookies)
	}
}

// setupCaptchaHandlers attaches captcha-specific handlers
func (d *DocDownloader) setupCaptchaHandlers() {
	d.collector.OnHTML("form#frmCaptcha", func(e *colly.HTMLElement) {
		d.HandleDocDownloadCaptchaForm(e)
	})
	d.collector.OnResponse(func(r *colly.Response) {
		d.HandleDocDownloadCaptchaResponse(r)
	})
}

// setupLinkExtractionHandlers attaches document extraction handlers
func (d *DocDownloader) setupLinkExtractionHandlers(documentLinks *[]DocumentLink) {
	d.collector.OnHTML("a[id*='DirectLink']", func(e *colly.HTMLElement) {
		docType := "NITDocuments" // default type
		// skip those docs which contains "Back"
		if strings.Contains(strings.ToLower(e.Text), "back") {
			return
		}
		// d.logger.Println("Possible Document Link!")
		// Check for <img> inside <a>
		img := e.DOM.Find("img")
		if img.Length() > 0 {
			src, exists := img.Attr("src")
			if !exists || !strings.Contains(strings.ToLower(src), "zip") {
				return // skip this <a> if <img src> doesn't contain "zip"
			}
			docType = "WorkDocumentsZip" // set type if <img src> contains zip
		}

		href := e.Attr("href")
		if href == "" {
			return
		}
		// fmt.Printf("Document Link Confirmed!\nText: %s\nURL: %s\n", strings.TrimSpace(e.Text), href)
		absURL := e.Request.AbsoluteURL(href)
		*documentLinks = append(*documentLinks, DocumentLink{
			URL:  absURL,
			Text: strings.TrimSpace(e.Text),
			Type: docType,
		})
	})

	d.collector.OnResponse(func(r *colly.Response) {
		d.logger.Printf("[%s][docDownload] visited %s, status: %d", d.state, r.Request.URL, r.StatusCode)

		// Save response body for debugging
		// filename := "debug_response.html"
		// if err := os.WriteFile(filename, r.Body, 0644); err != nil {
		// 	d.logger.Printf("[%s][docDownload] failed to save debug response: %v", d.state, err)
		// } else {
		// 	d.logger.Printf("[%s][docDownload] saved response to %s", d.state, filename)
		// }
	})

}

// Example usage in SolveDocCaptcha
func (d *DocDownloader) SolveDocCaptcha() error {
	docCaptchaURL := fmt.Sprintf("%s?component=docDownoad&page=FrontEndTenderDetails&service=direct&session=T",
		d.sess.BaseURL)
	d.logger.Printf("[%s][docCaptcha] visiting captcha URL: %s", d.state, docCaptchaURL)

	// config := DefaultConfig()

	d.clearHandlers()        // reset collector safely
	d.setupCaptchaHandlers() // attach captcha handlers

	if err := d.collector.Visit(docCaptchaURL); err != nil {
		return fmt.Errorf("failed to visit captcha URL: %w", err)
	}

	return d.waitForCaptchaSession(DefaultConfig())
}

// Example usage in extractDocumentLinks
func (d *DocDownloader) extractDocumentLinks(tenderURL string) []DocumentLink {
	d.logger.Printf("[%s][docDownload] extracting document links from: %s", d.state, tenderURL)

	var documentLinks []DocumentLink

	d.clearHandlers()                             // reset collector safely
	d.setupLinkExtractionHandlers(&documentLinks) // attach link extraction handlers

	if err := d.collector.Visit(tenderURL); err != nil {
		d.logger.Printf("[%s][docDownload] error visiting tender URL: %v", d.state, err)
	}

	return documentLinks
}

// hasDirectLinks checks if any docDownoad link contains DirectLink
func (d *DocDownloader) hasDirectLinks(documentLinks []DocumentLink) bool {
	for _, link := range documentLinks {
		if link.Type == "NITDocuments" && strings.Contains(link.URL, "DirectLink") {
			d.logger.Printf("[%s][docDownload] found DirectLink: %s", d.state, link.URL)
			return true
		}
	}
	return false
}

// processDirectLinks processes links that can be downloaded directly
func (d *DocDownloader) processDirectLinks(documentLinks []DocumentLink) {
	for _, link := range documentLinks {
		switch link.Type {
		case "NITDocuments":
			// Extract NIT document info (simplified)
			doc := NITDocument{
				DocumentName: link.Text,
				URL:          link.URL,
			}
			if doc.DocumentName == "" {
				doc.DocumentName = fmt.Sprintf("document_%d.pdf", time.Now().Unix())
			}
			d.NITDocs = append(d.NITDocs, doc)
			d.logger.Printf("[%s][docDownload] added NIT doc: %s", d.state, doc.DocumentName)
		case "WorkDocumentsZip":
			d.WorkItemZip = WorkItemDocument{
				DocumentName: "WorkItemDocs.zip", // or derive from link.Text
				URL:          link.URL,
			}
			d.logger.Printf("[%s][docDownload] added WorkItem zip: %s", d.state, link.URL)
		}
	}
}

// waitForCaptchaSession waits until captcha session is established
func (d *DocDownloader) waitForCaptchaSession(config Config) error {
	timeout := time.After(config.CaptchaTimeout)
	tick := time.NewTicker(config.PollInterval)
	defer tick.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for doc captcha solve")
		case <-tick.C:
			if d.sess.DocSessionEstablished() {
				d.logger.Printf("[%s][docDownload] captcha solved, session established", d.state)
				return nil
			}
		}
	}
}
