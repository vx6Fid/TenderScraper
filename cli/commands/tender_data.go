package commands

import (
	"log"

	"github.com/vx6fid/tender-scraper/scraper/extract"
	"github.com/vx6fid/tender-scraper/utils"
)

func ExtractTenderData(logger *log.Logger) error {
	runDate := utils.GetRunDate(false)

	for _, u := range utils.BaseURLs {
		// logger.Printf("--- Starting concurrent tender extraction for [%s] ---", u.State)

		totalJobs, err := utils.EstimateJobCount(u.State, runDate, false, "")
		if err != nil {
			logger.Printf("[%s] failed to estimate job count: %v", u.State, err)
			continue
		}

		optimalWorkers := utils.CalculateOptimalWorkers(totalJobs)
		extractor := extract.NewConcurrentExtractor(u.BaseURL, u.Domain, u.State, runDate, optimalWorkers)

		if err := extractor.ExtractTendersWithMultipleSessions(); err != nil {
			logger.Printf("[%s] concurrent extraction failed: %v", u.State, err)
			continue
		}

		logger.Printf("--- Completed [%s] ---", u.State)
	}

	return nil
}
