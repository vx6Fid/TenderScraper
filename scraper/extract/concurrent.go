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
	"time"

	"github.com/vx6fid/tender-scraper/session"
	"github.com/vx6fid/tender-scraper/utils"
)

// ConcurrentExtractor manages multiple workers with individual sessions
type ConcurrentExtractor struct {
	baseURL    string
	domain     string
	state      string
	runDate    string
	maxWorkers int
}

// WorkerSession represents a worker with its own session
type WorkerSession struct {
	WorkerID int
	Session  *session.Session
	Scraper  *DataScraper
	mu       sync.Mutex // Add mutex for thread safety
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
	log.Printf("--- Starting concurrent tender extraction for [%s] with %d workers ---", ce.state, ce.maxWorkers)

	// Load input CSV to count jobs
	rows, err := ce.loadInputCSV()
	if err != nil {
		return err
	}

	// Count valid jobs
	totalJobs := 0
	for i := 1; i < len(rows); i++ {
		if len(rows[i]) >= 5 {
			totalJobs++
		}
	}

	if totalJobs == 0 {
		log.Printf("[%s] No valid tender links found in CSV", ce.state)
		return nil
	}

	log.Printf("[%s] Found %d valid tender links to process with %d workers", ce.state, totalJobs, ce.maxWorkers)

	// Create job and result channels with proper buffering
	jobs := make(chan TenderInput, min(totalJobs, 100)) // Buffer to prevent blocking
	results := make(chan *TenderData, min(totalJobs, 100))

	var wg sync.WaitGroup
	var writerWg sync.WaitGroup

	// Setup output directory and writer
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

	// Start result writer
	writerWg.Add(1)
	go func() {
		defer writerWg.Done()
		written := 0
		for tenderData := range results {
			if tenderData != nil { // Add nil check
				if err := ce.writeToFile(outFile, tenderData); err != nil {
					log.Printf("[%s] Failed to write tender data: %v", ce.state, err)
				} else {
					written++
					if written%50 == 0 {
						log.Printf("[%s] Written %d/%d tenders", ce.state, written, totalJobs)
						outFile.Sync() // Periodic flush
					}
				}
			}
		}
		outFile.Sync() // Final flush
		log.Printf("[%s] Writer finished, wrote %d tenders", ce.state, written)
	}()

	// Create workers with individual sessions - with staggered initialization
	workers := make([]*WorkerSession, ce.maxWorkers)
	workerInitErrors := make(chan error, ce.maxWorkers)

	// Initialize workers in parallel with staggered delays to reduce server load
	var initWg sync.WaitGroup
	for i := 0; i < ce.maxWorkers; i++ {
		initWg.Add(1)
		go func(workerID int) {
			defer initWg.Done()

			// Stagger worker initialization to reduce server load
			time.Sleep(time.Duration(workerID) * 2 * time.Second)

			log.Printf("[%s] Initializing worker %d session...", ce.state, workerID)
			sess := session.NewSession(ce.baseURL, ce.state)

			// Add retry logic for session establishment
			maxRetries := 3
			var lastErr error

			for attempt := 1; attempt <= maxRetries; attempt++ {
				if err := sess.EstablishSession("ActiveTenders"); err != nil {
					lastErr = err
					log.Printf("[%s] Worker %d session attempt %d failed: %v", ce.state, workerID, attempt, err)
					if attempt < maxRetries {
						time.Sleep(time.Duration(attempt) * 5 * time.Second) // Exponential backoff
					}
					continue
				}

				// Session established successfully
				scraper := NewDataScraper(sess, ce.domain, ce.state, ce.runDate)
				workers[workerID] = &WorkerSession{
					WorkerID: workerID,
					Session:  sess,
					Scraper:  scraper,
				}
				log.Printf("[%s] Worker %d session established successfully", ce.state, workerID)
				return
			}

			// All attempts failed
			workerInitErrors <- fmt.Errorf("worker %d failed to establish session after %d attempts: %w", workerID, maxRetries, lastErr)
		}(i)
	}

	initWg.Wait()
	close(workerInitErrors)

	// Check for initialization errors
	var validWorkers []*WorkerSession
	for err := range workerInitErrors {
		log.Printf("[%s] Worker initialization error: %v", ce.state, err)
	}

	for _, worker := range workers {
		if worker != nil {
			validWorkers = append(validWorkers, worker)
		}
	}

	if len(validWorkers) == 0 {
		return fmt.Errorf("no workers could establish sessions")
	}

	log.Printf("[%s] Successfully initialized %d/%d workers", ce.state, len(validWorkers), ce.maxWorkers)

	// Start worker goroutines
	for _, worker := range validWorkers {
		wg.Add(1)
		go func(ws *WorkerSession) {
			defer wg.Done()
			ce.workerProcess(ws, jobs, results, totalJobs)
		}(worker)
	}

	// Enqueue all jobs with better error handling
	go func() {
		defer close(jobs)
		jobCount := 0

		for i := 1; i < len(rows); i++ {
			row := rows[i]
			if len(row) < 5 {
				continue
			}

			// Add validation for required fields
			serial := strings.TrimSpace(row[0])
			link := strings.TrimSpace(row[4])

			if serial == "" || link == "" {
				log.Printf("[%s] Skipping invalid row %d: missing serial or link", ce.state, i)
				continue
			}

			jobs <- TenderInput{
				Serial:       serial,
				Title:        strings.TrimSpace(row[1]),
				Organisation: strings.TrimSpace(row[2]),
				ClosingDate:  strings.TrimSpace(row[3]),
				Link:         link,
			}
			jobCount++

			// Progress logging
			if jobCount%100 == 0 || jobCount == totalJobs {
				log.Printf("[%s] Enqueued %d/%d jobs", ce.state, jobCount, totalJobs)
			}
		}
		log.Printf("[%s] Finished enqueuing all %d jobs", ce.state, jobCount)
	}()

	// Progress monitoring
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				log.Printf("[%s] Processing in progress with %d active workers...", ce.state, len(validWorkers))
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for all workers to complete
	wg.Wait()
	cancel() // Stop progress monitoring
	close(results)

	// Wait for writer to finish
	writerWg.Wait()

	log.Printf("--- Completed concurrent extraction for [%s] ---", ce.state)
	return nil
}

func (ce *ConcurrentExtractor) workerProcess(ws *WorkerSession, jobs <-chan TenderInput, results chan<- *TenderData, totalJobs int) {
	processed := 0
	progressInterval := calculateProgressInterval(totalJobs)

	log.Printf("[%s] Worker %d starting processing", ce.state, ws.WorkerID)

	defer func() {
		if r := recover(); r != nil {
			log.Printf("[%s] Worker %d recovered from panic: %v", ce.state, ws.WorkerID, r)
		}
	}()

	for tenderInput := range jobs {
		processed++

		if processed%progressInterval == 0 {
			log.Printf("[%s] Worker %d processed %d tenders", ce.state, ws.WorkerID, processed)
		}

		// Add worker-level mutex to prevent concurrent access to session
		// ws.mu.Lock()
		// Extract tender data using the worker's dedicated session
		start := time.Now()
		tenderData, err := ws.Scraper.ExtractSingleTender(tenderInput)
		elapsed := time.Since(start)
		log.Printf("[%s] Worker %d extracted tender %s in %s", ce.state, ws.WorkerID, tenderInput.Serial, elapsed)
		// ws.mu.Unlock()

		if err != nil {
			log.Printf("[%s_%s] Worker %d extraction failed: %v", ce.state, tenderInput.Serial, ws.WorkerID, err)
			// Still send a nil result to maintain count
			select {
			case results <- nil:
			case <-time.After(3 * time.Second):
				log.Printf("[%s] Worker %d timeout sending nil result for %s", ce.state, ws.WorkerID, tenderInput.Serial)
			}
			continue
		}

		// Send result with timeout
		select {
		case results <- tenderData:
			// Success
		case <-time.After(5 * time.Second): // Increased timeout
			log.Printf("[%s] Worker %d timeout sending tender %s", ce.state, ws.WorkerID, tenderInput.Serial)
		}
	}

	log.Printf("[%s] Worker %d finished processing %d tenders", ce.state, ws.WorkerID, processed)
}

func (ce *ConcurrentExtractor) loadInputCSV() ([][]string, error) {
	fileName := fmt.Sprintf("%s_Links.csv", ce.state)
	filePath := fmt.Sprintf("TenderData/Links/%s", ce.runDate)
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

// This method will be the same as in your original code
func (ce *ConcurrentExtractor) convertToUtilsTender(data *TenderData) utils.Tender {
	// Create a new DataScraper instance just for conversion
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
