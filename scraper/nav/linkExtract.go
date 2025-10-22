package nav

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/vx6fid/tender-scraper/scraper/nav/active"
	"github.com/vx6fid/tender-scraper/session"
	"github.com/vx6fid/tender-scraper/utils"
	"github.com/vx6fid/tender-scraper/utils/browser"
	types "github.com/vx6fid/tender-scraper/utils/types"
)

type LinkExtractor struct {
	// runDate string
}

func NewLinkExtractor() *LinkExtractor {
	return &LinkExtractor{
		// runDate: runDate,
	}
}

func (le *LinkExtractor) ActiveLinksBrowser() error {
	urlCh := make(chan types.URLS)
	errCh := make(chan error, len(utils.BaseURLs))
	var wg sync.WaitGroup

	// --- Start bounded worker pool ---
	for i := 0; i < utils.MaxSessionParallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for u := range urlCh {
				b := browser.NewBrowser()
				defer b.Close()
				if err := active.Run(b, u); err != nil {
					errCh <- err
				}
			}
		}()
	}

	// Feed URLs to workers
	for _, u := range utils.BaseURLs {
		urlCh <- u
	}
	close(urlCh)

	wg.Wait()
	close(errCh)

	// Log and return first error if any
	var firstErr error
	for err := range errCh {
		if err != nil {
			log.Println(err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
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
			headers := []string{"S.No", "TenderID", "PageNo", "Title", "Organisation Chain", "Tender Stage", "Link", "Unique Identifier"}
			stateWriter := NewPastWriter(u.State, headers, utils.StageName[stage])
			failedWriter := NewFailedWriter(u.State, utils.StageName[stage])
			defer stateWriter.Close()
			defer failedWriter.Close()

			// Channel of date range jobs
			jobs := make(chan [2]time.Time, len(dateRanges))
			var workers sync.WaitGroup

			// Launch workers
			numWorkers := utils.CalculateWorkersPastLinks(len(dateRanges))
			for range numWorkers {
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
