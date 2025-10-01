package commands

import (
	"log"

	"github.com/vx6fid/tender-scraper/scraper/nav"
	"github.com/vx6fid/tender-scraper/utils"
)

func ExtractPastTenderLinks(logger *log.Logger) error {
	runDate := utils.GetRunDate(false)
	fromStr := "01/01/2024"
	toStr := "30/09/2025"

	tenderType := utils.GiveStageName()
	linkExtractor := nav.NewLinkExtractor(runDate)
	linkExtractor.PastTenders(fromStr, toStr, 7, tenderType)

	return nil
}
