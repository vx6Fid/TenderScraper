package commands

import (
	"log"
	"time"

	"github.com/vx6fid/tender-scraper/scraper/extract"
	"github.com/vx6fid/tender-scraper/utils"
)

func ExtractTenderData(logger *log.Logger) error {
	runDate := utils.GetRunDate(false)
	loc := time.FixedZone("IST", 19800) // UTC+5:30
	updatedAt := time.Now().In(loc).Truncate(time.Second)

	for _, u := range utils.BaseURLs {
		// logger.Printf("--- Starting concurrent tender extraction for [%s] ---", u.State)

		totalJobs, err := utils.EstimateJobCount(u.State, runDate, false, "")
		if err != nil {
			logger.Printf("[%s] failed to estimate job count: %v", u.State, err)
			continue
		}

		optimalWorkers := utils.CalculateOptimalWorkers(totalJobs)
		extractor := extract.NewConcurrentExtractor(u.BaseURL, u.Domain, u.State, runDate, optimalWorkers, updatedAt)

		if err := extractor.ExtractTendersWithMultipleSessions(); err != nil {
			logger.Printf("[%s] concurrent extraction failed: %v", u.State, err)
			continue
		}

		logger.Printf("--- Completed [%s] ---", u.State)
	}

	return nil
}
