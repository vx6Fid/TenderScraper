package nav

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/vx6fid/tender-scraper/scraper/nav/active"
	"github.com/vx6fid/tender-scraper/session"
	session_browser "github.com/vx6fid/tender-scraper/session-browser"
	"github.com/vx6fid/tender-scraper/utils"
	"github.com/vx6fid/tender-scraper/utils/browser"
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

func (le *LinkExtractor) ActiveLinksBrowser() error {
	fmt.Println("Starting browser...")
	for _, u := range utils.BaseURLs {
		// 1. Launch browser (headful in dev)
		b := browser.NewBrowser()

		// 2. Establish session
		page, err := session_browser.EstablishSession(b, u.BaseURL, u.State)
		if err != nil {
			return fmt.Errorf("session establishment failed: %w", err)
		}

		page = b.MustPage(u.BaseURL + "?component=%24DirectLink&page=FrontEndTendersByOrganisation&service=direct&session=T")
		page.MustWaitLoad()
		page.MustWaitElementsMoreThan("table#table tr", 1)
		page.MustWaitStable()

		rows := page.MustElements("table#table tr")
		currCount := len(rows)
		fmt.Printf("Current rows: %d\n", currCount)

		// 3. Continue with scraping logic here
		if err := active.Run(u.State, page); err != nil {
			return fmt.Errorf("active links extraction failed: %w", err)
		}

	}

	return nil
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
