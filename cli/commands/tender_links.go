package commands

import (
	"log"

	"github.com/vx6fid/tender-scraper/scraper/nav"
	"github.com/vx6fid/tender-scraper/utils"
)

func ExtractTenderLinks(logger *log.Logger) error {
	runDate := utils.GetRunDate(false)
	linkExtractor := nav.NewLinkExtractor(runDate)

	if err := linkExtractor.Run(); err != nil {
		logger.Printf("link extraction failed: %v", err)
		return err
	}

	return nil
}
