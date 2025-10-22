package extract

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vx6fid/tender-scraper/session"
	"github.com/vx6fid/tender-scraper/utils"
	types "github.com/vx6fid/tender-scraper/utils/types"
)

type ConcurrentExtractor struct {
	baseURL          string
	domain           string
	state            string
	runDate          string
	maxWorkers       int
	failedTenderLogs *FailedTenderWriter
}

type WorkerSession struct {
	WorkerID int
	Session  *session.Session
	Scraper  *DataScraper
	mu       sync.Mutex
}

func NewConcurrentExtractor(baseURL, domain, state, runDate string, maxWorkers int) *ConcurrentExtractor {
	return &ConcurrentExtractor{
		baseURL:    baseURL,
		domain:     domain,
		state:      state,
		runDate:    runDate,
		maxWorkers: maxWorkers,
	}
}

func (ce *ConcurrentExtractor) ExtractTendersWithMultipleSessions() error {
	fmt.Print("\n")
	log.Printf("--- Starting concurrent tender extraction for [%s] with %d workers ---", ce.state, ce.maxWorkers)

	rows, err := ce.loadInputCSV()
	if err != nil {
		return err
	}

	// count jobs
	totalJobs := 0
	for i := 1; i < len(rows); i++ {
		// for _, content := range rows[i] {
		// 	fmt.Println(content)
		// }
		if len(rows[i]) >= 7 {
			totalJobs++
		}
	}
	if totalJobs == 0 {
		log.Printf("[%s] No valid tender links found in CSV", ce.state)
		return nil
	}
	log.Printf("[%s] Found %d valid tender links to process with %d workers", ce.state, totalJobs, ce.maxWorkers)

	// channels
	jobs := make(chan TenderInput, min(totalJobs, ce.maxWorkers*3))
	results := make(chan *TenderData, min(totalJobs, ce.maxWorkers*50))

	// job counter + cancel context
	var remainingJobs int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// setup output writer
	dir := filepath.Join("TenderData/Tenders", ce.runDate)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	fileName := filepath.Join(dir, "tender.jsonl")
	outFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// writer goroutine
	var writerWg sync.WaitGroup
	writerWg.Add(1)
	go func() {
		defer writerWg.Done()
		written := 0
		const flushInterval = 500 // flush after every 500 writes

		for tenderData := range results {
			if tenderData == nil {
				continue
			}
			if err := ce.writeToFile(outFile, tenderData); err != nil {
				log.Printf("[%s] Failed to write tender data: %v", ce.state, err)
			} else {
				written++
				if written%flushInterval == 0 {
					if err := outFile.Sync(); err != nil {
						log.Printf("[%s] Failed to sync file: %v", ce.state, err)
					}
				}
				// Log progress every 100 tenders to reduce console spam
				if written%100 == 0 || written == totalJobs {
					log.Printf("[%s] Written %d/%d tenders", ce.state, written, totalJobs)
				}
			}
		}

		// final sync after all writes
		if err := outFile.Sync(); err != nil {
			log.Printf("[%s] Failed to sync file at end: %v", ce.state, err)
		}

		log.Printf("[%s] Writer finished, wrote %d tenders", ce.state, written)
	}()

	// failed tender logger
	ce.failedTenderLogs = NewFailedTenderWriter(ce.state)
	defer ce.failedTenderLogs.Close()

	// semaphore: limit session creation
	initLimiter := make(chan struct{}, utils.MaxSessionParallel)

	// worker wg
	var wg sync.WaitGroup

	// launch workers
	workerCount := min(totalJobs, ce.maxWorkers) // spin only as many as needed
	for w := range workerCount {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// abort early if no jobs
			select {
			case <-ctx.Done():
				log.Printf("[%s] Worker %d skipped initialization: no tenders left", ce.state, workerID)
				return
			default:
			}

			maxWorkerAttempts := 3
			for attempt := 1; attempt <= maxWorkerAttempts; attempt++ {
				initLimiter <- struct{}{}
				sess := session.NewSession(ce.baseURL, ce.state)

				maxRetries := 3
				var lastErr error
				ok := false
				for r := 1; r <= maxRetries; r++ {
					// check cancel before heavy work
					select {
					case <-ctx.Done():
						<-initLimiter
						log.Printf("[%s] Worker %d aborted during session retry", ce.state, workerID)
						return
					default:
					}
					start := time.Now()
					err := sess.EstablishSession("ActiveTenders")
					if err != nil {
						lastErr = err
						log.Printf("[%s] Worker %d session attempt %d failed: %v", ce.state, workerID, r, err)
						time.Sleep(time.Duration(r) * 5 * time.Second)
						continue
					} else {
						fmt.Print("\n========================================================================\n")
						log.Printf("[%s] Worker %d session established in %v", ce.state, workerID, time.Since(start))
						fmt.Print("\n========================================================================\n")
					}
					ok = true
					break
				}
				<-initLimiter

				if ok {
					// still jobs left?
					if atomic.LoadInt32(&remainingJobs) == 0 {
						log.Printf("[%s] Worker %d skipping start: no jobs remaining", ce.state, workerID)
						return
					}
					scraper := NewDataScraper(sess, ce.domain, ce.state, ce.runDate)
					ws := &WorkerSession{WorkerID: workerID, Session: sess, Scraper: scraper}
					ce.workerProcess(ctx, ws, jobs, results, &remainingJobs, cancel, totalJobs)
					return
				}

				log.Printf("[%s] Worker %d failed to establish session (attempt %d/%d): %v",
					ce.state, workerID, attempt, maxWorkerAttempts, lastErr)
			}
			log.Printf("[%s] Worker %d giving up after failed attempts", ce.state, workerID)
		}(w)
	}

	// enqueue jobs
	go func() {
		defer close(jobs)
		jobCount := 0
		for i := 1; i < len(rows); i++ {
			row := rows[i]
			if len(row) < 7 {
				continue
			}
			serial := strings.TrimSpace(row[0])
			link := strings.TrimSpace(row[5])
			if serial == "" || link == "" {
				continue
			}
			atomic.AddInt32(&remainingJobs, 1)
			jobs <- TenderInput{
				Serial:           serial,
				Title:            strings.TrimSpace(row[1]),
				Organisation:     strings.TrimSpace(row[2]),
				ClosingDate:      strings.TrimSpace(row[4]),
				Link:             link,
				UniqueIdentifier: row[6],
			}
			jobCount++
		}
		log.Printf("[%s] Finished enqueuing %d jobs", ce.state, jobCount)
		if jobCount == 0 {
			cancel()
		}
	}()

	// wait for workers + writer
	wg.Wait()
	cancel()
	close(results)
	writerWg.Wait()

	log.Printf("--- Completed concurrent extraction for [%s] ---", ce.state)
	return nil
}

func (ce *ConcurrentExtractor) workerProcess(
	ctx context.Context,
	ws *WorkerSession,
	jobs <-chan TenderInput,
	results chan<- *TenderData,
	remainingJobs *int32,
	cancel context.CancelFunc,
	totalJobs int,
) {
	processed := 0
	progressInterval := calculateProgressInterval(totalJobs)
	// log.Printf("[%s] Worker %d starting processing", ce.state, ws.WorkerID)

	defer func() {
		if r := recover(); r != nil {
			log.Printf("[%s] Worker %d recovered from panic: %v", ce.state, ws.WorkerID, r)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[%s] Worker %d stopping: context canceled", ce.state, ws.WorkerID)
			return
		case tenderInput, ok := <-jobs:
			if !ok {
				// log.Printf("[%s] Worker %d finished processing %d tenders", ce.state, ws.WorkerID, processed)
				return
			}
			processed++
			if processed%progressInterval == 0 {
				// log.Printf("[%s] Worker %d processed %d tenders", ce.state, ws.WorkerID, processed)
			}

			// start := time.Now()
			tenderData, err := ws.Scraper.ExtractSingleTender(tenderInput)
			// elapsed := time.Since(start)
			// log.Printf("[%s] Worker %d extracted tender %s in %s", ce.state, ws.WorkerID, tenderInput.Serial, elapsed)

			if err != nil {
				log.Printf("[%s_%s] Worker %d extraction failed: %v", ce.state, tenderInput.Serial, ws.WorkerID, err)
				if ce.failedTenderLogs != nil {
					ce.failedTenderLogs.WriteFailure(tenderInput.Serial, tenderInput.Link, err.Error())
				}
			} else {
				results <- tenderData
			}

			// decrement remaining
			if atomic.AddInt32(remainingJobs, -1) == 0 {
				log.Printf("[%s] All jobs completed, canceling workers...", ce.state)
				cancel()
			}
		}
	}
}

func (ce *ConcurrentExtractor) loadInputCSV() ([][]string, error) {
	fileName := "active.csv"
	filePath := fmt.Sprintf("TenderData/Links/%s/%s", ce.runDate, ce.state)
	inputPath := filepath.Join(filePath, fileName)

	inFile, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open links CSV at %s: %w", inputPath, err)
	}
	defer inFile.Close()

	reader := csv.NewReader(inFile)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read links CSV: %w", err)
	}
	if len(rows) <= 1 {
		return nil, fmt.Errorf("no data rows found in %s", inputPath)
	}

	return rows, nil
}

func (ce *ConcurrentExtractor) writeToFile(file *os.File, data *TenderData) error {
	tender := ce.convertToUtilsTender(data)
	return WriteJSONLToFile(file, tender)
}

// Create a new DataScraper instance just for conversion
func (ce *ConcurrentExtractor) convertToUtilsTender(data *TenderData) types.Tender {
	ds := &DataScraper{state: ce.state}
	return ds.ConvertToUtilsTender(data)
}

// Helper functions
func calculateProgressInterval(totalJobs int) int {
	switch {
	case totalJobs <= 10:
		return 1
	case totalJobs <= 100:
		return 10
	case totalJobs <= 1000:
		return 50
	default:
		return 100
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
