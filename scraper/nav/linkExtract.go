package nav

import (
	"log"
	"sync"

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

// func (le *LinkExtractor) Run() error {
// 	log.Println("=== Link extraction started ===")

// 	for _, u := range le.baseURLs {
// 		log.Printf(">>> [%s] Extraction started (Domain=%s, BaseURL=%s)", u.State, u.Domain, u.BaseURL)

// 		// Step 1: Establish session
// 		log.Printf("[%s] Establishing session...", u.State)
// 		sess := session.NewSession(u.BaseURL, u.State)
// 		if err := sess.EstablishSession(); err != nil {
// 			log.Printf("[%s] [ERROR] Failed to establish session: %v", u.State, err)
// 			continue
// 		}
// 		log.Printf("[%s] Session established.", u.State)

// 		// Step 2: Fetch total pages
// 		log.Printf("[%s] Fetching total pages...", u.State)
// 		totalPages, err := utils.FetchTotalPages(sess, u.BaseURL)
// 		if err != nil {
// 			log.Printf("[%s] [ERROR] Failed to fetch total pages: %v", u.State, err)
// 			continue
// 		}
// 		log.Printf("[%s] Total pages found: %d", u.State, totalPages)

// 		workers := utils.CalculateOptimalWorkers(totalPages)
// 		log.Printf("[%s] Worker pool size = %d", u.State, workers)

// 		// Step 3: Initialize CSV writer goroutine
// 		log.Printf("[%s] Initializing CSV writer...", u.State)
// 		csvWriter := NewCSVWriter(u.State) // returns struct with rows channel
// 		defer csvWriter.Close()            // ensures flush and file close
// 		log.Printf("[%s] CSV writer ready.", u.State)

// 		// Step 4: Prepare pages channel
// 		pages := make(chan int, totalPages)
// 		for i := 1; i <= totalPages; i++ {
// 			pages <- i
// 		}
// 		close(pages)
// 		log.Printf("[%s] Page channel populated with %d pages.", u.State, totalPages)

// 		// Step 5: Launch workers
// 		var wg sync.WaitGroup
// 		log.Printf("[%s] Launching %d workers...", u.State, workers)
// 		for w := 0; w < workers; w++ {
// 			wg.Add(1)
// 			go func(workerID int) {
// 				defer wg.Done()

// 				log.Printf("[%s] Worker %d establishing session...", u.State, workerID)
// 				wsess := session.NewSession(u.BaseURL, u.State)
// 				if err := wsess.EstablishSession(); err != nil {
// 					log.Printf("[%s] [ERROR] Worker %d failed session: %v", u.State, workerID, err)
// 					return
// 				}
// 				log.Printf("[%s] Worker %d session established.", u.State, workerID)

// 				for pageNum := range pages {
// 					log.Printf("[%s] Worker %d started scraping page %d...", u.State, workerID, pageNum)
// 					scraper := NewSearchScraper(wsess, u.Domain, u.State, pageNum)

// 					// Instead of writing directly, push row to channel
// 					scraper.SetRowHandler(func(row []string) {
// 						csvWriter.WriteRow(row)
// 					})

// 					if err := scraper.Scrape(); err != nil {
// 						log.Printf("[%s] [ERROR] Worker %d page %d failed: %v", u.State, workerID, pageNum, err)
// 					} else {
// 						log.Printf("[%s] Worker %d completed page %d.", u.State, workerID, pageNum)
// 					}
// 				}

// 				log.Printf("[%s] Worker %d finished all assigned pages.", u.State, workerID)
// 			}(w)
// 		}

// 		wg.Wait()
// 		log.Printf("<<< [%s] Extraction completed (Pages=%d, Workers=%d)", u.State, totalPages, workers)
// 	}

// 	log.Println("=== Link extraction finished ===")
// 	return nil
// }

func (le *LinkExtractor) Run() error {
	log.Println("=== Link extraction started ===")

	// Validate sessions in parallel

	for _, u := range le.baseURLs {
		log.Printf(">>> [%s] Extraction started", u.State)

		// Step 1: Establish session
		log.Printf("[%s] Establishing session...", u.State)
		mainSess := session.NewSession(u.BaseURL, u.State)
		if err := mainSess.EstablishSession(); err != nil {
			log.Printf("[%s] [ERROR] Failed to establish session: %v", u.State, err)
			continue
		}
		log.Printf("[%s] Session established.", u.State)

		// Step 2: Fetch total pages
		log.Printf("[%s] Fetching total pages...", u.State)
		totalPages, err := utils.FetchTotalPages(mainSess, u.BaseURL, u.Domain)
		if err != nil {
			log.Printf("[%s] [ERROR] Failed to fetch total pages: %v", u.State, err)
			continue
		}
		log.Printf("[%s] Total pages found: %d", u.State, totalPages)

		workers := utils.CalculateOptimalWorkers(totalPages)
		log.Printf("[%s] Worker pool size = %d", u.State, workers)

		// Step 3: Initialize CSV writer
		csvWriter := NewCSVWriter(u.State)
		defer csvWriter.Close()

		// Optional: deduplication map
		// seen := make(map[string]struct{})
		// var seenMu sync.Mutex

		// Step 4: Prepare pages channel
		pages := make(chan int, totalPages)
		for i := 1; i <= totalPages; i++ {
			pages <- i
		}
		close(pages)

		// Step 5: Launch workers
		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for pageNum := range pages {
					var pageWg sync.WaitGroup
					pageWg.Add(1)

					scraper := NewSearchScraper(mainSess, u.Domain, u.State, pageNum)
					scraper.SetRowHandler(func(row []string) {
						// deduplicate by link
						// link := row[6]
						// seenMu.Lock()
						// if _, exists := seen[link]; !exists {
						csvWriter.WriteRow(row)
						// 	seen[link] = struct{}{}
						// }
						// seenMu.Unlock()
					})

					go func() {
						defer pageWg.Done()
						if err := scraper.Scrape(); err != nil {
							log.Printf("[%s] Worker %d page %d failed: %v", u.State, workerID, pageNum, err)
						} else {
							log.Printf("[%s] Worker %d completed page %d", u.State, workerID, pageNum)
						}
					}()
					pageWg.Wait()
				}

				log.Printf("[%s] Worker %d finished all assigned pages.", u.State, workerID)
			}(w)
		}

		wg.Wait()
		log.Printf("<<< [%s] Extraction completed (Pages=%d, Workers=%d)", u.State, totalPages, workers)
	}

	log.Println("=== Link extraction finished ===")
	return nil
}

// func (le *LinkExtractor) Run() error {
// 	log.Println("=== Link extraction started ===")

// 	var wg sync.WaitGroup

// 	for _, u := range le.baseURLs {
// 		wg.Add(1)
// 		go func(u utils.URLS) {
// 			defer wg.Done()

// 			log.Printf(">>> [%s] Extraction started", u.State)

// 			// Step 1: Establish session
// 			log.Printf("[%s] Establishing session...", u.State)
// 			sess := session.NewSession(u.BaseURL, u.State)
// 			if err := sess.EstablishSession(); err != nil {
// 				log.Printf("[%s] [ERROR] Failed to establish session: %v", u.State, err)
// 				return
// 			}
// 			log.Printf("[%s] Session established.", u.State)

// 			// Step 2: Fetch total pages
// 			log.Printf("[%s] Fetching total pages...", u.State)
// 			totalPages, err := utils.FetchTotalPages(sess, u.BaseURL, u.Domain)
// 			if err != nil {
// 				log.Printf("[%s] [ERROR] Failed to fetch total pages: %v", u.State, err)
// 				return
// 			}
// 			log.Printf("[%s] Total pages found: %d", u.State, totalPages)

// 			// Step 3: Initialize CSV writer
// 			csvWriter := NewCSVWriter(u.State)
// 			defer csvWriter.Close()

// 			// Deduplication map (per baseURL)
// 			seen := make(map[string]struct{})

// 			// Step 4: Sequentially scrape pages
// 			for pageNum := 1; pageNum <= totalPages; pageNum++ {
// 				log.Printf("[%s] Scraping page %d...", u.State, pageNum)

// 				scraper := NewSearchScraper(sess, u.Domain, u.State, pageNum)
// 				scraper.SetRowHandler(func(row []string) {
// 					link := row[6]
// 					if _, exists := seen[link]; !exists {
// 						csvWriter.WriteRow(row)
// 						seen[link] = struct{}{}
// 					}
// 				})

// 				if err := scraper.Scrape(); err != nil {
// 					log.Printf("[%s] [ERROR] Page %d failed: %v", u.State, pageNum, err)
// 				} else {
// 					log.Printf("[%s] Completed page %d", u.State, pageNum)
// 				}
// 			}

// 			log.Printf("<<< [%s] Extraction completed (Pages=%d)", u.State, totalPages)
// 		}(u)
// 	}

// 	wg.Wait()
// 	log.Println("=== Link extraction finished ===")
// 	return nil
// }

func (le *LinkExtractor) ActiveLinks() {
	var wg sync.WaitGroup

	for _, u := range le.baseURLs {
		wg.Add(1)
		go func(u utils.URLS) {
			defer wg.Done()

			sess := session.NewSession(u.BaseURL, u.State)
			if err := sess.EstablishSession(); err != nil {
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
	var wg sync.WaitGroup

	for _, u := range le.baseURLs {
		wg.Add(1)
		go func(u utils.URLS) {
			defer wg.Done()

			sess := session.NewSession(u.BaseURL, u.State)
			if err := sess.EstablishSession(); err != nil {
				log.Printf("[%s] failed to establish session: %v", u.State, err)
			}

			scraper := NewCorrScraper(sess, u.Domain, u.State)
			if err := scraper.ScrapeCorrigendum(); err != nil {
				log.Printf("[%s] scraping failed: %v", u.State, err)
			}
		}(u)
	}
	wg.Wait()
}
