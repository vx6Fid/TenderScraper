package main

import (
	"fmt"
	"log"

	"github.com/vx6fid/tender-scraper/scraper"
	"github.com/vx6fid/tender-scraper/scraper/nav"
	"github.com/vx6fid/tender-scraper/session"
)

func main() {
	log.Println("Starting Rajasthan Tender Scraper")

	// baseURLs := []string{
	// 	"https://etenders.gov.in/eprocure/app",
	// 	"https://coalindiatenders.nic.in/nicgep/app",
	// 	"https://iocletenders.nic.in/nicgep/app",
	// 	"https://cpcletenders.nic.in/nicgep/app",
	// 	"https://eprocurebel.co.in/nicgep/app",
	// 	"https://eprocurentpc.nic.in/nicgep/app",
	// 	"https://eprocuregsl.nic.in/nicgep/app",
	// 	"https://eprocurehsl.nic.in/nicgep/app",
	// 	"https://eprocuremdl.nic.in/nicgep/app",
	// 	"https://www.eprocuremidhani.nic.in/nicgep/app",
	// 	"https://eprocuregrse.co.in/nicgep/app",
	// 	"https://eprocurebhel.co.in/nicgep/app",
	// }

	fmt.Println("Scrape Tender links or tender data--")
	fmt.Println("1.Tender links")
	fmt.Println("2.Tender data")
	var choice int
	fmt.Scan(&choice)

	baseURL := "https://etender.up.nic.in/nicgep/app"
	domain := "etender.up.nic.in"

	if choice == 1 {
		// Uttar Pradesh

		sess := session.NewSession(baseURL)

		if err := sess.EstablishSession(); err != nil {
			log.Fatalf("failed to establish session: %v", err)
		}

		scraper := nav.NewTenderScraper(sess, domain)

		if err := scraper.ScrapeActiveTenders(); err != nil {
			log.Fatalf("Scraping failed: %v", err)
		}

	} else if choice == 2 {

		sess := session.NewSession(baseURL)

		if err := sess.EstablishSession(); err != nil {
			log.Fatalf("failed to establish session: %v", err)
		}

		scraper := scraper.NewTenderDataScraper(sess, domain, "UttarPradesh")
		if err := scraper.ExtractTenderData(); err != nil {
			log.Fatalf("Scraping failed: %v", err)
		}
	} else {
		fmt.Println("Invalid choice")
	}

	log.Println("Scraping completed successfully")
}
