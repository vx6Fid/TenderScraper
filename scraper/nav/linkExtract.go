package nav

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/vx6fid/tender-scraper/session"
	"github.com/vx6fid/tender-scraper/utils"
)

type LinkExtractor struct {
	runDate  string
	baseURLs []utils.URLS
}

func NewLinkExtractor(runDate string, baseURLs []utils.URLS) *LinkExtractor {
	return &LinkExtractor{
		runDate:  runDate,
		baseURLs: baseURLs,
	}
}

func (le *LinkExtractor) Run() error {
	log.Println("=== Link extraction started ===")

	// ---------------------------------------------
	// Step 0: Parallel session validation
	// ---------------------------------------------
	type validatedSession struct {
		state  string
		domain string
		sess   *session.Session
	}

	var validSessions []validatedSession
	var mu sync.Mutex     // protects validSessions slice
	var wg sync.WaitGroup // waits for all session validation goroutines

	// Limit concurrent session validation to avoid overwhelming the system
	sem := make(chan struct{}, utils.MaxSessionParallel)

	for _, u := range le.baseURLs {
		wg.Add(1)
		go func(u utils.URLS) {
			defer wg.Done()

			sem <- struct{}{}        // acquire slot
			defer func() { <-sem }() // release slot after session validation

			s := session.NewSession(u.BaseURL, u.State)
			if err := s.EstablishSession("ActiveTenders"); err != nil {
				log.Printf("[%s] [ERROR] Failed to establish session: %v", u.State, err)
				return
			}

			log.Printf("[%s] Session established.", u.State)

			mu.Lock()
			validSessions = append(validSessions, validatedSession{state: u.State, sess: s, domain: u.Domain})
			mu.Unlock()
		}(u)
	}
	wg.Wait() // wait until all sessions are validated

	if len(validSessions) == 0 {
		return fmt.Errorf("no sessions could be established")
	}

	// ---------------------------------------------
	// Step 1: For each validated session, launch worker pool for scraping
	// ---------------------------------------------
	for _, vs := range validSessions {
		log.Printf(">>> [%s] Starting extraction", vs.state)

		// Fetch total pages for the session
		totalPages, err := utils.FetchTotalPages(vs.sess, vs.sess.BaseURL, vs.domain)
		if err != nil {
			log.Printf("[%s] [ERROR] Failed to fetch total pages: %v", vs.state, err)
			continue
		}
		log.Printf("[%s] Total pages: %d", vs.state, totalPages)

		// Determine worker pool size dynamically
		workers := utils.CalculateOptimalWorkers(totalPages)
		log.Printf("[%s] Worker pool size = %d", vs.state, workers)

		// Initialize CSV writer for this session
		csvWriter := NewCSVWriter(vs.state)
		defer csvWriter.Close()

		// Optional deduplication map
		// seen := make(map[string]struct{})
		// var seenMu sync.Mutex

		// Prepare pages channel
		pages := make(chan int, totalPages)
		for i := 1; i <= totalPages; i++ {
			pages <- i
		}
		close(pages)

		// Launch workers for scraping pages
		var wgWorkers sync.WaitGroup
		for w := 0; w < workers; w++ {
			wgWorkers.Add(1)
			go func(workerID int) {
				defer wgWorkers.Done()
				for pageNum := range pages {
					scraper := NewSearchScraper(vs.sess, vs.domain, vs.state, pageNum)
					scraper.SetRowHandler(func(row []string) {
						// Deduplication example:
						// link := row[6]
						// seenMu.Lock()
						// if _, exists := seen[link]; !exists {
						csvWriter.WriteRow(row)
						// 	seen[link] = struct{}{}
						// }
						// seenMu.Unlock()
					})

					const maxRetries = 3
					for attempt := 1; attempt <= maxRetries; attempt++ {
						if err := scraper.Scrape(); err != nil {
							log.Printf("[%s] Worker %d page %d failed (attempt %d/%d): %v",
								vs.state, workerID, pageNum, attempt, maxRetries, err)
							time.Sleep(time.Duration(attempt) * 100 * time.Millisecond) // backoff
							continue
						}
						log.Printf("[%s] Worker %d completed page %d", vs.state, workerID, pageNum)
						break
					}
				}

				log.Printf("[%s] Worker %d finished all assigned pages.", vs.state, workerID)
			}(w)
		}

		wgWorkers.Wait() // wait until all pages for this session are scraped
		log.Printf("<<< [%s] Extraction completed (Pages=%d, Workers=%d)", vs.state, totalPages, workers)
	}

	log.Println("=== Link extraction finished ===")
	return nil
}

func (le *LinkExtractor) ActiveLinks() {
	var wg sync.WaitGroup

	for _, u := range le.baseURLs {
		wg.Add(1)
		go func(u utils.URLS) {
			defer wg.Done()

			sess := session.NewSession(u.BaseURL, u.State)
			if err := sess.EstablishSession("ActiveTenders"); err != nil {
				log.Printf("[%s] failed to establish session: %v", u.State, err)
			}

			scraper := NewActiveScraper(sess, u.Domain, u.State)
			if err := scraper.ScrapeActiveTenders(); err != nil {
				log.Printf("[%s] scraping failed: %v", u.State, err)
			}
		}(u)
	}
	wg.Wait()
}

func (le *LinkExtractor) Corrigendums() {
	sem := make(chan struct{}, utils.MaxSessionParallel)
	var wg sync.WaitGroup

	for _, u := range le.baseURLs {
		wg.Add(1)
		go func(u utils.URLS) {
			defer wg.Done()

			sess := session.NewSession(u.BaseURL, u.State)

			// Acquire slot ONLY for captcha solving
			sem <- struct{}{}
			if err := sess.EstablishSession("CorrigendumTenders"); err != nil {
				log.Printf("[%s] failed to establish session: %v", u.State, err)
			}
			<-sem // release slot immediately after captcha solved

			scraper := NewCorrScraper(sess, u.Domain, u.State)
			if err := scraper.ScrapeCorrigendum(); err != nil {
				log.Printf("[%s] scraping failed: %v", u.State, err)
			}
		}(u)
	}
	wg.Wait()
	log.Println("=== Corrigendum extraction finished ===")
}

func (le *LinkExtractor) PastTenders() {
	sem := make(chan struct{}, utils.MaxSessionParallel)
	var wg sync.WaitGroup

	from, to := utils.GetDateRange()
	for _, u := range le.baseURLs {
		wg.Add(1)
		go func(u utils.URLS) {
			defer wg.Done()

			sess := session.NewSession(u.BaseURL, u.State)

			// Acquire slot ONLY for captcha solving
			sem <- struct{}{}
			if err := sess.EstablishTenderStatusSession("6", from, to); err != nil {
				log.Printf("[%s] failed to establish session: %v", u.State, err)
			}
			<-sem // release slot immediately after captcha solved

			headers := []string{"S.No", "TenderID", "PageNo", "Title", "Organisation Chain", "Tender Stage", "Link"}
			scraper, err := NewPastScraper(sess, u.Domain, u.State, headers)
			if err != nil {
				log.Printf("[%s] failed to create scraper: %v", u.State, err)
				return
			}
			if err := scraper.Run(); err != nil {
				log.Printf("[%s] scraping failed: %v", u.State, err)
			}
		}(u)
	}
	wg.Wait()
	log.Println("=== Past Tenders extraction finished ===")
}
