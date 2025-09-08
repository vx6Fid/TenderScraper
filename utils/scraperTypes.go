package utils

import (
	"encoding/csv"
	"os"

	"github.com/gocolly/colly/v2"
)

type ScrapLinks struct {
	collector          *colly.Collector
	baseURL            string
	captchaSolved      bool
	sessionEstablished bool
	resultsFound       bool
	activeTendersURL   string
	totalTenders       int
	scrapedTenders     int
	currentPage        int
	nextButtonURL      string
	csvFile            *os.File
	csvWriter          *csv.Writer
}
