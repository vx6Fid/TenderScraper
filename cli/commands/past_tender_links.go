package commands

import (
	"fmt"
	"log"

	"github.com/vx6fid/tender-scraper/scraper/nav"
	"github.com/vx6fid/tender-scraper/utils"
)

func ExtractPastTenderLinks(logger *log.Logger) error {
	// runDate := utils.GetRunDate(false)
	fromStr := "01/01/2024"
	toStr := "02/01/2024"

	// tenderType := utils.GiveStageName()
	blockSize := 2
	linkExtractor := nav.NewLinkExtractor()
	keys := []string{
		"2",
		"3",
		"4",
		"5",
		"6",
	}
	for _, key := range keys {
		fmt.Println("Extracting links for \"" + utils.StageName[key] + "\"")
		linkExtractor.PastTenders(fromStr, toStr, blockSize, key)
	}
	return nil
}
