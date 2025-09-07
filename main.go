package main

import (
	"log"

	"github.com/vx6fid/tender-scraper/scraper"
	"github.com/vx6fid/tender-scraper/scraper/nav"
)

func main() {
	log.Println("Starting Rajasthan Tender Scraper")

	// Create collector
	// baseURL := "https://eproc.rajasthan.gov.in/nicgep/app"
	// c := scraper.NewCollector("eproc.rajasthan.gov.in")

	// baseURL := "https://coalindiatenders.nic.in/nicgep/app"
	// c := scraper.NewCollector("coalindiatenders.nic.in")

	// Uttar Pradesh
	baseURL := "https://etender.up.nic.in/nicgep/app"
	c := scraper.NewCollector("etender.up.nic.in")

	// Create and run scraper
	tenderScraper := nav.NewTenderScraper(c, baseURL)

	if err := tenderScraper.ScrapeActiveTenders(); err != nil {
		log.Fatalf("Scraping failed: %v", err)
	}

	log.Println("Scraping completed successfully")
}
