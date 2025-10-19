package nav

import (
	"encoding/csv"
	"fmt"
	"net/url"
	"strconv"
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
	rowsScraped := 0
	var scrapeErr error

	s.collector.OnHTML(`tr#informal, tr[id^="informal"]`, func(e *colly.HTMLElement) {
		cells := e.ChildTexts("td")
		if len(cells) < 6 {
			return
		}
		rowsScraped++

		raw := e.ChildText("td:nth-child(5)")
		title := e.ChildText("td:nth-child(5) a")
		link := e.ChildAttr("td:nth-child(5) a", "href")

		if strings.HasPrefix(link, "/") {
			u, _ := url.Parse(s.baseURL)
			link = u.Scheme + "://" + u.Host + link
		}

		// raw = utils.CleanField(raw)
		tenderID := utils.ExtractTenderID(raw)

		row := []string{
			strings.TrimSuffix(cells[0], "."), // S.No
			strconv.Itoa(s.currentPage),       // Page No
			cells[1],                          // e-Published Date
			cells[2],                          // Closing Date
			cells[3],                          // Opening Date
			title,
			tenderID,
			link,
			cells[5], // Organisation Chain
			raw,      // Unique Identifier
		}

		if s.rowHandler != nil {
			s.rowHandler(row)
		}
	})

	s.collector.OnError(func(r *colly.Response, err error) {
		scrapeErr = fmt.Errorf("request failed: %w", err)
	})

	url := utils.BuildPageURLRaw(s.baseURL, s.currentPage)
	fmt.Println(url)
	if err := s.collector.Visit(url); err != nil {
		return fmt.Errorf("visit failed: %w", err)
	}

	// Treat no rows scraped as an error
	if rowsScraped == 0 && scrapeErr == nil {
		scrapeErr = fmt.Errorf("no rows scraped on page %d", s.currentPage)
	}

	return scrapeErr
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

		link := e.ChildAttr("td:nth-child(5) a", "href")
		if strings.HasPrefix(link, "/") {
			u, _ := url.Parse(s.baseURL)
			link = u.Scheme + "://" + u.Host + link
		}

		// raw = utils.CleanField(raw)
		tenderID := utils.ExtractTenderID(raw)

		row := []string{
			sno,      // S.No
			cells[1], // e-Published Date
			cells[2], // Closing Date
			cells[3], // Opening Date
			title,    // Title
			tenderID, // Ref.No./TenderID
			link,     // Tender link
			cells[5], // Organisation Chain
			raw,      // Unique Identifier
		}

		if s.rowHandler != nil {
			s.rowHandler(row)
		}
	})

	url := utils.BuildPageURLRaw(s.baseURL, s.currentPage)
	return s.collector.Visit(url)
}
