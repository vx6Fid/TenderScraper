package commands

import (
	"log"

	"github.com/vx6fid/tender-scraper/scraper/nav"
	"github.com/vx6fid/tender-scraper/utils"
)

func ExtractCorrigendumLinks(logger *log.Logger) error {
	runDate := utils.GetRunDate(false)
	linkExtractor := nav.NewLinkExtractor(runDate)
	linkExtractor.Corrigendums()
	return nil
}
