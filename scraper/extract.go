package scraper

import (
	"encoding/csv"
	"log"
	"os"

	"github.com/gocolly/colly/v2"
	"github.com/vx6fid/tender-scraper/session"
)

type TenderDataScraper struct {
	collector          *colly.Collector
	captchaSolved      bool
	sessionEstablished bool
	activeTendersURL   string
	csvFile            *os.File
	csvWriter          *csv.Writer
}

func NewTenderDataScraper(sess *session.Session, domain string, state string) *TenderDataScraper {
	collector := sess.NewCollector(domain)
	// collector.AllowURLRevisit = true
	return &TenderDataScraper{
		collector:        collector,
		activeTendersURL: sess.ActiveTendersURL,
	}
}

func (ts *TenderDataScraper) ExtractTenderData() error {
	log.Println("Starting tender scraping process with correct session flow.")

	// use state to open the respective file

	// open CSV
	// file, err := os.Create("tenders.csv")
	// if err != nil {
	// 	return fmt.Errorf("failed to create CSV file: %w", err)
	// }
	// ts.csvFile = file
	// ts.csvWriter = csv.NewWriter(file)

	// // write header
	// ts.csvWriter.Write([]string{"Serial Number", "Title", "Organisation", "Closing Date", "Link"})
	// ts.csvWriter.Flush()

	// defer func() {
	// 	if ts.csvWriter != nil {
	// 		ts.csvWriter.Flush()
	// 	}
	// 	if ts.csvFile != nil {
	// 		ts.csvFile.Close()
	// 	}
	// }()

	// Visit Each Link from UttarPradeshLinks.csv
	// Clik on View Tender Details
	// Extract Data from it, also on the same page we have Corregendum links, scrape.

	return nil
}
