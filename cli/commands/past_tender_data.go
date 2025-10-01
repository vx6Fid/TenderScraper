package commands

import (
	"fmt"
	"log"

	"github.com/vx6fid/tender-scraper/scraper/pastTenders"
	"github.com/vx6fid/tender-scraper/utils"
)

func ExtractPastTenderData(logger *log.Logger) error {
	runDate := utils.GetRunDate(true)
	dir := fmt.Sprintf("TenderData/PastLinks/%s", runDate)
	tenderType := utils.GiveStageName()

	if err := pastTenders.Run(dir, runDate, tenderType); err != nil {
		return fmt.Errorf("error running past tenders: %w", err)
	}

	return nil
}
