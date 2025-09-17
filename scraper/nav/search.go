package nav

import (
	"encoding/csv"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/gocolly/colly/v2"
	"github.com/vx6fid/tender-scraper/session"
	"github.com/vx6fid/tender-scraper/utils"
)

type SearchScraper struct {
	collector   *colly.Collector
	state       string
	baseURL     string
	csvWriter   *csv.Writer
	currentPage int

	rowHandler func([]string)
}

func NewSearchScraper(sess *session.Session, domain string, state string, currentPage int) *SearchScraper {
	c := sess.NewCollector(domain)
	return &SearchScraper{
		collector:   c,
		state:       state,
		baseURL:     sess.BaseURL,
		currentPage: currentPage,
	}
}

func (s *SearchScraper) SetRowHandler(fn func([]string)) {
	s.rowHandler = fn
}

func (s *SearchScraper) Scrape() error {
	s.collector.OnHTML(`tr#informal, tr[id^="informal"]`, func(e *colly.HTMLElement) {
		cells := e.ChildTexts("td")
		if len(cells) < 6 {
			return
		}

		// Column 5: title + RefNo + Tender ID
		raw := e.ChildText("td:nth-child(5)")
		link := e.ChildAttr("td:nth-child(5) a", "href")

		// Clean link: prepend base URL if relative
		if strings.HasPrefix(link, "/") {
			link = "https://etender.up.nic.in" + link
		}

		sno := strings.TrimSuffix(cells[0], ".")
		// Extract title: only inside <a>
		title := e.ChildText("td:nth-child(5) a")

		// The leftover text (Ref.No./TenderID)
		leftover := strings.TrimSpace(strings.Replace(raw, title, "", 1))

		row := []string{
			sno,      // S.No
			cells[1], // e-Published Date
			cells[2], // Closing Date
			cells[3], // Opening Date
			title,    // Title
			leftover, // Ref.No./TenderID
			link,     // Tender link
			cells[5], // Organisation Chain
		}

		if s.rowHandler != nil {
			s.rowHandler(row)
		}
	})

	s.collector.OnScraped(func(r *colly.Response) {
		log.Printf("[%s] Finished scraping page %d", s.state, s.currentPage)
		// s.csvWriter.Flush()
	})

	url := utils.BuildPageURLRaw(s.baseURL, s.currentPage)
	if err := s.collector.Visit(url); err != nil {
		return fmt.Errorf("visit failed: %w", err)
	}

	return nil
}

func (s *SearchScraper) ScrapeWithMutex(mu *sync.Mutex) error {
	s.collector.OnHTML(`tr#informal, tr[id^="informal"]`, func(e *colly.HTMLElement) {
		cells := e.ChildTexts("td")
		if len(cells) < 6 {
			return
		}

		sno := strings.TrimSuffix(cells[0], ".")
		title := e.ChildText("td:nth-child(5) a")
		raw := e.ChildText("td:nth-child(5)")
		leftover := strings.TrimSpace(strings.Replace(raw, title, "", 1))

		link := e.ChildAttr("td:nth-child(5) a", "href")
		if strings.HasPrefix(link, "/") {
			link = "https://etender.up.nic.in" + link
		}

		row := []string{
			sno,      // S.No
			cells[1], // e-Published Date
			cells[2], // Closing Date
			cells[3], // Opening Date
			title,    // Title
			leftover, // Ref.No./TenderID
			link,     // Tender link
			cells[5], // Organisation Chain
		}

		if s.rowHandler != nil {
			s.rowHandler(row)
		}
	})

	url := utils.BuildPageURLRaw(s.baseURL, s.currentPage)
	return s.collector.Visit(url)
}
