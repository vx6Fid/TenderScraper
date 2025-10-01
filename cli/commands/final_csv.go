package commands

import (
	"log"

	"github.com/vx6fid/tender-scraper/utils"
)

func PrepareFinalCSV(logger *log.Logger) error {
	runDate := utils.GetRunDate(false)

	for _, u := range utils.BaseURLs {
		if err := utils.FinalCSV(runDate, u.State); err != nil {
			logger.Printf("[%s] CSV generation failed: %v", u.State, err)
		} else {
			logger.Printf("[%s] CSV file generated successfully", u.State)
		}
	}

	return nil
}
