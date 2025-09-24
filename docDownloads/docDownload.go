package docdownload

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/vx6fid/tender-scraper/session"
	"github.com/vx6fid/tender-scraper/utils"
)

// DocDownloader handles NIT and Zip downloads using a single collector
type DocDownloader struct {
	sess            *session.Session
	state           string
	logger          *log.Logger
	collector       *colly.Collector
	NITDocs         []NITDocument
	WorkItemZip     WorkItemDocument
	CorrigendumDocs []CorrigendumDocument
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
func (d *DocDownloader) Run(tenderID string, tenderURL string, corrigendumLinks []utils.CorrLinks) error {
	d.logger.Printf("[%s][docDownload] solving doc captcha", d.state)
	if err := d.SolveDocCaptcha(); err != nil {
		return err
	}

	d.logger.Printf("[%s][docDownload] visiting tender page: %s", d.state, tenderURL)

	// Extract document links
	documentLinks, viewLinks := d.extractDocumentLinks(tenderURL)
	if len(documentLinks) == 0 {
		d.logger.Printf("[%s][docDownload] no document links found", d.state)
		return nil
	}

	// Must have DirectLinks now
	if !d.hasDirectLinks(documentLinks) {
		return fmt.Errorf("[%s][docDownload] unexpected: no DirectLinks found even after captcha", d.state)
	}

	d.processDirectLinks(documentLinks, corrigendumLinks)

	// now handle corrigendum view links
	d.visitViewLinks(viewLinks)

	// Download files locally
	if err := d.downloadFiles(); err != nil {
		return err
	}

	if err := d.processAndUploadDocs(tenderID, "tenderbharat"); err != nil {
		return err
	}

	return nil
}

// visitViewLinks visits each View Link page sequentially and optionally saves HTML for debugging
func (d *DocDownloader) visitViewLinks(viewLinks []string) {
	for _, url := range viewLinks {
		d.logger.Printf("[%s][docDownload] visiting View Link page: %s", d.state, url)

		c := d.collector.Clone()
		cookies := d.collector.Cookies(d.sess.BaseURL)
		if len(cookies) > 0 {
			c.SetCookies(d.sess.BaseURL, cookies)
		}

		c.OnResponse(func(r *colly.Response) {
			if strings.Contains(strings.ToLower(r.Request.URL.String()), "directlink") &&
				strings.Contains(strings.ToLower(r.Request.URL.String()), "page=frontendtenderdetails") {
				filename := fmt.Sprintf("viewlink_debug_%d.html", time.Now().UnixNano())
				if err := os.WriteFile(filename, r.Body, 0644); err != nil {
					d.logger.Printf("[%s][docDownload] failed to save debug HTML: %v", d.state, err)
				} else {
					d.logger.Printf("[%s][docDownload] saved debug HTML to %s", d.state, filename)
				}
			}
		})

		if err := c.Visit(url); err != nil {
			d.logger.Printf("[%s][docDownload] failed to visit View Link: %v", d.state, err)
			continue
		}

		time.Sleep(100 * time.Millisecond)
	}
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
func (d *DocDownloader) setupLinkExtractionHandlers(documentLinks *[]DocumentLink, viewLinks *[]string) {
	d.collector.OnHTML("a[id*='DirectLink']", func(e *colly.HTMLElement) {
		href := e.Attr("href")
		if href == "" {
			return
		}
		absURL := e.Request.AbsoluteURL(href)

		// Detect if this is a View Link by title
		title := e.Attr("title")
		if strings.Contains(strings.ToLower(title), "view more details") {
			return
		}
		if strings.Contains(strings.ToLower(title), "view corrigendum") {
			d.logger.Printf("[%s][docDownload] found View Link: %s", d.state, absURL)
			*viewLinks = append(*viewLinks, absURL)

			// Visit the View Link to enable corrigendum downloads
			// err := d.collector.Visit(absURL)
			// if err != nil {
			// 	d.logger.Printf("[%s][docDownload] failed to visit View Link: %v", d.state, err)
			// }

			return // skip further processing for View Links
		}

		// Skip links that contain "Back"
		if strings.Contains(strings.ToLower(e.Text), "back") {
			return
		}

		// Determine document type
		docType := "NITDocuments"
		img := e.DOM.Find("img")
		if img.Length() > 0 {
			src, exists := img.Attr("src")
			if exists && strings.Contains(strings.ToLower(src), "zip") {
				docType = "WorkDocumentsZip"
			} else {
				return
			}
		}

		// Append regular document link
		*documentLinks = append(*documentLinks, DocumentLink{
			URL:  absURL,
			Text: strings.TrimSpace(e.Text),
			Type: docType,
		})

		// if docType == "NITDocuments" {
		// 	d.logger.Printf("[%s][docDownload] attributes for NIT link (%s):", d.state, absURL)

		// 	if len(e.DOM.Nodes) > 0 {
		// 		for _, attr := range e.DOM.Nodes[0].Attr {
		// 			fmt.Printf("  %s = %s\n", attr.Key, attr.Val)
		// 		}
		// 	}
		// }

	})

	d.collector.OnResponse(func(r *colly.Response) {
		d.logger.Printf("[%s][docDownload] visited %s, status: %d", d.state, r.Request.URL, r.StatusCode)
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
func (d *DocDownloader) extractDocumentLinks(tenderURL string) ([]DocumentLink, []string) {
	d.logger.Printf("[%s][docDownload] extracting document links from: %s", d.state, tenderURL)

	var documentLinks []DocumentLink
	var viewLinks []string

	d.clearHandlers()                                         // reset collector safely
	d.setupLinkExtractionHandlers(&documentLinks, &viewLinks) // attach link extraction handlers

	if err := d.collector.Visit(tenderURL); err != nil {
		d.logger.Printf("[%s][docDownload] error visiting tender URL: %v", d.state, err)
	}

	return documentLinks, viewLinks
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
func (d *DocDownloader) processDirectLinks(documentLinks []DocumentLink, corrigendumLinks []utils.CorrLinks) {
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
	for _, corrLink := range corrigendumLinks {
		d.CorrigendumDocs = append(d.CorrigendumDocs, CorrigendumDocument{
			DocumentName: corrLink.Name,
			Type:         corrLink.Type,
			URL:          corrLink.Link,
		})
		d.logger.Printf("[%s][docDownload] added Corrigendum doc: %s", d.state, corrLink.Name)

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
