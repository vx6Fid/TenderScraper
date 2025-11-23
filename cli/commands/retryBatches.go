package commands

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/vx6fid/tender-scraper/utils"
)

type BatchFailure struct {
	Batch  int
	Failed []DocumentConfig
}

func RunBatch(configs []DocumentConfig, logger *log.Logger) ([]DocumentConfig, error) {
	sem := make(chan struct{}, utils.MaxDownloadWorkers)
	var wg sync.WaitGroup

	var failedMu sync.Mutex
	var failed []DocumentConfig

	for _, config := range configs {
		cfg := config
		sem <- struct{}{}
		wg.Add(1)

		go func() {
			defer func() { <-sem; wg.Done() }()

			ctx := context.Background()
			_, err := RetryProcessTender(ctx, cfg, logger, 3, 500*time.Millisecond)
			if err != nil {
				// ONLY configs with error go into failed list (your rule: Q2 = A)
				failedMu.Lock()
				failed = append(failed, cfg)
				failedMu.Unlock()
			}
		}()
	}

	wg.Wait()
	return failed, nil
}

func RetryProcessTender(
	ctx context.Context,
	cfg DocumentConfig,
	logger *log.Logger,
	attempts int,
	delay time.Duration,
) (DownloadStats, error) {

	var finalStats DownloadStats
	var lastErr error
	backoff := delay

	for i := range attempts {
		finalStats, lastErr = ProcessTender(ctx, cfg, logger)
		if lastErr == nil {
			return finalStats, nil
		}

		// discard stats produced by this failed attempt
		finalStats = DownloadStats{}

		if i < attempts-1 {
			time.Sleep(backoff)
			backoff *= 2
		}
	}

	return finalStats, lastErr
}
