package commands

import (
	"log"

	"github.com/vx6fid/tender-scraper/scraper/nav"
)

func ExtractTenderLinks(logger *log.Logger) error {
	// runDate := utils.GetRunDate(false)
	linkExtractor := nav.NewLinkExtractor()
	if err := linkExtractor.ActiveLinksBrowser(); err != nil {
		logger.Printf("link extraction failed: %v", err)
		return err
	}

	return nil
}
