package main

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/joho/godotenv"
	"github.com/vx6fid/tender-scraper/scraper"
	"github.com/vx6fid/tender-scraper/scraper/nav"
	"github.com/vx6fid/tender-scraper/session"
	"github.com/vx6fid/tender-scraper/utils"
)

func main() {
	log.Println("--- Starting Tender Scraper ---")

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	baseURLs := []utils.URLS{
		{BaseURL: "https://etenders.gov.in/eprocure/app", State: "CentralGovernment", Domain: "etenders.gov.in"},
		{BaseURL: "https://coalindiatenders.nic.in/nicgep/app", State: "CoalIndia", Domain: "coalindiatenders.nic.in"},
		{BaseURL: "https://iocletenders.nic.in/nicgep/app", State: "IndianOil", Domain: "iocletenders.nic.in"},
		{BaseURL: "https://cpcletenders.nic.in/nicgep/app", State: "CPCIL", Domain: "cpcletenders.nic.in"},
		{BaseURL: "https://eprocurebel.co.in/nicgep/app", State: "BEL", Domain: "eprocurebel.co.in"},
		{BaseURL: "https://eprocurentpc.nic.in/nicgep/app", State: "NTPC", Domain: "eprocurentpc.nic.in"},
		{BaseURL: "https://eprocuregsl.nic.in/nicgep/app", State: "GSL", Domain: "eprocuregsl.nic.in"},
		{BaseURL: "https://eprocurehsl.nic.in/nicgep/app", State: "HSL", Domain: "eprocurehsl.nic.in"},
		{BaseURL: "https://eprocuremdl.nic.in/nicgep/app", State: "MDL", Domain: "eprocuremdl.nic.in"},
		{BaseURL: "https://www.eprocuremidhani.nic.in/nicgep/app", State: "Midhani", Domain: "www.eprocuremidhani.nic.in"},
		{BaseURL: "https://eprocuregrse.co.in/nicgep/app", State: "GRSE", Domain: "eprocuregrse.co.in"},
		{BaseURL: "https://eprocurebhel.co.in/nicgep/app", State: "BHEL", Domain: "eprocurebhel.co.in"},
	}

	fmt.Println("--- Choose one of the following ---")
	fmt.Println("1.Tender links")
	fmt.Println("2.Tender data")
	var choice int
	fmt.Print("Enter your choice: ")
	fmt.Scan(&choice)

	switch choice {
	case 1:
		var wg sync.WaitGroup

		for _, u := range baseURLs {
			wg.Add(1)
			go func(u utils.URLS) {
				defer wg.Done()

				sess := session.NewSession(u.BaseURL, u.State)
				if err := sess.EstablishSession(); err != nil {
					log.Printf("[%s] failed to establish session: %v", u.State, err)
					return
				}

				scraper := nav.NewTenderScraper(sess, u.Domain, u.State)
				if err := scraper.ScrapeActiveTenders(); err != nil {
					log.Printf("[%s] scraping failed: %v", u.State, err)
				}
			}(u)
		}

		wg.Wait()
	case 2:
		baseURL := "https://etender.up.nic.in/nicgep/app"
		domain := "etender.up.nic.in"
		state := "UttarPradesh"
		sess := session.NewSession(baseURL, state)

		if err := sess.EstablishSession(); err != nil {
			log.Fatalf("failed to establish session: %v", err)
		}

		scraper := scraper.NewTenderDataScraper(sess, domain, state)
		if err := scraper.ExtractTenderData(); err != nil {
			log.Fatalf("Scraping failed: %v", err)
		}
	default:
		fmt.Println("Invalid choice")
		os.Exit(1)
	}

	log.Println("Scraping completed successfully")
}
