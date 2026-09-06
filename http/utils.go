package main

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/vx6fid/tender-scraper/cli/commands"
	types "github.com/vx6fid/tender-scraper/utils/types"
)

// --------------------
// Task Status Tracking
// --------------------

type TaskStatus struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

var (
	taskStore = make(map[string]TaskStatus)
	mu        sync.RWMutex
)

const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusFailed    = "failed"
	StatusCompleted = "completed"
)

// --------------------
// Environment Loader
// --------------------

func LoadEnvOrFatal() {
	// In containers, env vars are injected directly (docker compose env_file),
	// so a missing .env file is fine as long as the required vars are present.
	if err := godotenv.Load(); err != nil {
		if os.Getenv("POSTGRES_CONN_STRING") == "" || os.Getenv("RABBITMQ_URL") == "" {
			log.Fatalf("no .env file and required env vars missing: %v", err)
		}
		// Logger isn't initialized yet here; main logs this after slog.Init().
	}
}

// --------------------
// Status Helpers
// --------------------

func setStatus(id, s string, errMsg ...string) {
	mu.Lock()
	defer mu.Unlock()

	ts := taskStore[id]
	ts.ID = id
	ts.Status = s
	if len(errMsg) > 0 {
		ts.Error = errMsg[0]
	}
	taskStore[id] = ts
}

// --------------------
// Worker Pool
// --------------------

var (
	jobQueue chan func()
	wg       sync.WaitGroup
	stopCh   chan struct{}
)

func StartWorkerPool(n int) {
	jobQueue = make(chan func(), 1000)
	stopCh = make(chan struct{})

	for i := range n {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				select {
				case job, ok := <-jobQueue:
					if !ok {
						return
					}
					// recover from panics in job
					func() {
						defer func() {
							if r := recover(); r != nil {
								log.Printf("worker %d: panic recovered: %v", workerID, r)
							}
						}()
						job()
					}()
				case <-stopCh:
					return
				}
			}
		}(i)
	}
}

// Call during shutdown
func StopWorkerPool() {
	close(stopCh)
	// Closes jobQueue --> stop accepting jobs
	close(jobQueue)
	wg.Wait()
}

// --------------------
// Tender Download Worker
// --------------------

func runTenderDownload(ctx context.Context, id, url string, corrLinks []types.CorrLinks) {
	setStatus(id, StatusRunning)
	// create a child context with timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	_, err := commands.ProcessTender(
		ctx,
		commands.DocumentConfig{
			ID:               id,
			TenderURL:        url,
			CorrigendumLinks: corrLinks,
			UpdatedAt:        time.Now(),
		},
		log.Default(),
	)

	if err != nil {
		setStatus(id, StatusFailed, err.Error())
		return
	}

	setStatus(id, StatusCompleted)
}
