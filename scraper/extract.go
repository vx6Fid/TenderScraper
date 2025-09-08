package scraper

import (
	"encoding/csv"
	"os"

	"github.com/gocolly/colly/v2"
)

type TenderDataScraper struct {
	collector          *colly.Collector
	captchaSolved      bool
	sessionEstablished bool
	activeTendersURL   string
	csvFile            *os.File
	csvWriter          *csv.Writer
}

func (ts *TenderDataScraper) ExtractTenderData() error {

	return nil
}
