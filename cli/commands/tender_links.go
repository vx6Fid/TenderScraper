package commands

import (
	"fmt"
	"log"

	"github.com/vx6fid/tender-scraper/scraper/nav"
	"github.com/vx6fid/tender-scraper/utils"
)

func ExtractTenderLinks(logger *log.Logger) error {
	runDate := utils.GetRunDate(false)
	linkExtractor := nav.NewLinkExtractor(runDate)
	fmt.Println("Extracting tender links...")
	if err := linkExtractor.ActiveLinks(); err != nil {
		logger.Printf("link extraction failed: %v", err)
		return err
	}

	return nil
}
