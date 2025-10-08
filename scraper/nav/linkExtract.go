package nav

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/vx6fid/tender-scraper/session"
	"github.com/vx6fid/tender-scraper/utils"
	types "github.com/vx6fid/tender-scraper/utils/types"
)

type LinkExtractor struct {
	runDate string
}

func NewLinkExtractor(runDate string) *LinkExtractor {
	return &LinkExtractor{
		runDate: runDate,
	}
}

func (le *LinkExtractor) Run() error {
	log.Println("=== Link extraction started ===")

	type validatedSession struct {
		state  string
		domain string
		sess   *session.Session
	}

	var validSessions []validatedSession
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Writers for failed sessions even if session establishment fails
	failedSessionsWriter := NewFailedSessWriter()
	defer failedSessionsWriter.Close()

	sem := make(chan struct{}, utils.MaxSessionParallel)

	for _, u := range utils.BaseURLs {
		wg.Add(1)
		go func(u types.URLS) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			s := session.NewSession(u.BaseURL, u.State)
			if err := s.EstablishSession("ActiveTenders"); err != nil {
				log.Printf("[%s] [ERROR] Failed to establish session: %v", u.State, err)
				// Write to failed sessions file
				failedSessionsWriter.WriteFailure(u.State, u.BaseURL, err.Error())
				return
			}

			log.Printf("[%s] Session established.", u.State)
			mu.Lock()
			validSessions = append(validSessions, validatedSession{state: u.State, sess: s, domain: u.Domain})
			mu.Unlock()
		}(u)
	}
	wg.Wait()

	if len(validSessions) == 0 {
		return fmt.Errorf("no sessions could be established for any state")
	}

	// Process each validated session
	for _, vs := range validSessions {
		fmt.Print("\n")
		log.Printf(">>> [%s] Starting extraction", vs.state)

		totalPages, err := utils.FetchTotalPages(vs.sess, vs.sess.BaseURL, vs.domain)
		if err != nil {
			log.Printf("[%s] [ERROR] Failed to fetch total pages: %v", vs.state, err)
			totalPages = 1
		}
		log.Printf("[%s] Total pages: %d", vs.state, totalPages)

		workers := utils.CalculateOptimalWorkers(totalPages)
		log.Printf("[%s] Worker pool size = %d", vs.state, workers)

		// Writers
		failedWriter := NewFailedSearchWriter(vs.state)
		csvWriter := NewCSVWriter(vs.state)
		defer failedWriter.Close()
		defer csvWriter.Close()

		pages := make(chan int, totalPages)
		for i := 1; i <= totalPages; i++ {
			pages <- i
		}
		close(pages)

		var wgWorkers sync.WaitGroup
		for w := 0; w < workers; w++ {
			wgWorkers.Add(1)
			go func(workerID int) {
				defer wgWorkers.Done()
				for pageNum := range pages {
					scraper := NewSearchScraper(vs.sess, vs.domain, vs.state, pageNum)
					scraper.SetRowHandler(func(row []string) {
						csvWriter.WriteRow(row)
					})

					lastErr := scraper.Scrape()
					if lastErr != nil {
						// Write immediately to failed page file
						failedWriter.WriteFailure(pageNum, lastErr.Error())
					}
				}
				log.Printf("[%s] Worker %d finished all assigned pages.", vs.state, workerID)
			}(w)
		}

		wgWorkers.Wait()
		log.Printf("<<< [%s] Extraction completed (Pages=%d, Workers=%d)", vs.state, totalPages, workers)
	}

	fmt.Print("\n")
	log.Println("=== Link extraction finished ===")
	return nil
}

func (le *LinkExtractor) ActiveLinks() {
	var wg sync.WaitGroup

	for _, u := range utils.BaseURLs {
		wg.Add(1)
		go func(u types.URLS) {
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

	failedSessWriter := NewFailedCorrigendumWriter("Sessions")

	for _, u := range utils.BaseURLs {
		wg.Add(1)
		go func(u types.URLS) {
			defer wg.Done()

			failedWriter := NewFailedCorrigendumWriter(u.State)

			sess := session.NewSession(u.BaseURL, u.State)

			// Acquire semaphore for session establishment
			sem <- struct{}{}
			// log.Printf("[%s] Starting Session Establishment\n", u.State)
			err := sess.EstablishSession("CorrigendumTenders")
			<-sem // release immediately after establishment

			if err != nil {
				log.Printf("[%s] failed to establish session: %v", u.State, err)
				failedSessWriter.WriteFailure(u.State, fmt.Sprintf("Session failed: %v", err))
				return
			}
			log.Printf("[%s] Session Established\n", u.State)

			scraper := NewCorrScraper(sess, u.Domain, u.State, failedWriter)
			if err := scraper.ScrapeCorrigendum(); err != nil {
				log.Printf("[%s] scraping failed: %v", u.State, err)
				logMessage := fmt.Sprintf("[%s] scraping failed: %v", u.State, err)
				failedWriter.WriteFailure(u.State, logMessage)
			}
		}(u)
	}
	wg.Wait()
	fmt.Print("\n")
	log.Println("=== Corrigendum extraction finished ===")
}

func (le *LinkExtractor) PastTenders(fromStr, toStr string, chunkSize int, stage string) {
	sem := make(chan struct{}, utils.MaxSessionParallel) // global session limiter
	var wg sync.WaitGroup

	// Parse input dates
	from, err := time.Parse("02/01/2006", fromStr)
	if err != nil {
		log.Fatalf("invalid from date: %v", err)
	}
	to, err := time.Parse("02/01/2006", toStr)
	if err != nil {
		log.Fatalf("invalid to date: %v", err)
	}

	// Split into ranges
	if chunkSize <= 0 {
		chunkSize = 7
	}
	dateRanges := utils.SplitDateRange(from, to, chunkSize)

	// Loop over states
	for _, u := range utils.BaseURLs {
		wg.Add(1)
		go func(u types.URLS) {
			defer wg.Done()

			// One writer per state
			headers := []string{"S.No", "TenderID", "PageNo", "Title", "Organisation Chain", "Tender Stage", "Link"}
			stateWriter := NewPastWriter(u.State, headers, utils.StageName[stage])
			failedWriter := NewFailedWriter(u.State, utils.StageName[stage])
			defer stateWriter.Close()
			defer failedWriter.Close()

			// Channel of date range jobs
			jobs := make(chan [2]time.Time, len(dateRanges))
			var workers sync.WaitGroup

			// Launch workers
			numWorkers := utils.CalculateWorkersPastLinks(len(dateRanges))
			for i := 0; i < numWorkers; i++ {
				workers.Add(1)
				go func() {
					defer workers.Done()
					for dr := range jobs {
						fromFormatted := utils.FormatDate(dr[0])
						toFormatted := utils.FormatDate(dr[1])

						sess := session.NewSession(u.BaseURL, u.State)

						// Acquire slot ONLY for captcha solving
						sem <- struct{}{}
						if err := sess.EstablishTenderStatusSession(u.State, stage, fromFormatted, toFormatted); err != nil {
							log.Printf("[%s] failed to establish session for range %s-%s: %v",
								u.State, fromFormatted, toFormatted, err)
							failedWriter.WriteFailure(fromFormatted, toFormatted, err.Error())
							<-sem
							continue
						}
						<-sem // release slot

						log.Printf("[%s] worker started range %s - %s", u.State, fromFormatted, toFormatted)

						// Past Scraper
						scraper, err := NewPastScraper(sess, u.Domain, u.State, nil, stateWriter, failedWriter)
						if err != nil {
							log.Printf("[%s] failed to create scraper: %v", u.State, err)
							failedWriter.WriteFailure(fromFormatted, toFormatted, err.Error())
							continue
						}

						if err := scraper.Run(); err != nil {
							log.Printf("[%s] scraping failed for range %s-%s: %v",
								u.State, fromFormatted, toFormatted, err)
							failedWriter.WriteFailure(fromFormatted, toFormatted, err.Error())
						}

						log.Printf("[%s] worker finished range %s - %s", u.State, fromFormatted, toFormatted)
					}
				}()
			}

			// Feed jobs
			for _, dr := range dateRanges {
				jobs <- dr
			}
			close(jobs)
			workers.Wait()
			fmt.Print("\n")
			log.Printf("[%s] all date ranges completed for [%s]", u.State, utils.StageName[stage])
			fmt.Print("\n\n")
		}(u)
	}

	wg.Wait()
	log.Println("=== Past Tenders extraction finished ===")
}
