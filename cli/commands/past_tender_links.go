package commands

import (
	"fmt"
	"log"

	"github.com/vx6fid/tender-scraper/scraper/nav"
	"github.com/vx6fid/tender-scraper/utils"
)

func ExtractPastTenderLinks(logger *log.Logger) error {
	runDate := utils.GetRunDate(false)
	fromStr := "01/01/2025"
	toStr := "30/09/2025"

	// tenderType := utils.GiveStageName()
	blockSize := 7
	linkExtractor := nav.NewLinkExtractor(runDate)
	keys := []string{"2", "3", "4", "5", "6"}
	for _, key := range keys {
		fmt.Println("Extracting links for \"" + utils.StageName[key] + "\"")
		linkExtractor.PastTenders(fromStr, toStr, blockSize, key)
	}
	return nil
}
