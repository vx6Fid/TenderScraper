package main

import (
	"fmt"
	"log"
	"os"
	"slices"
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
		{BaseURL: "https://eprocure.gov.in/eprocure/app", State: "eProcurementCentralGovernment", Domain: "eprocure.gov.in"},
		{BaseURL: "https://defproc.gov.in/nicgep/app", State: "MinistryOfDefence", Domain: "defproc.gov.in"},
		{BaseURL: "https://pmgsytenders.gov.in/nicgep/app", State: "PMGSY", Domain: "pmgsytenders.gov.in"},
		{BaseURL: "https://etenders.gov.in/eprocure/app", State: "PSU", Domain: "etenders.gov.in"},
		{BaseURL: "https://coalindiatenders.nic.in/nicgep/app", State: "CoalIndia", Domain: "coalindiatenders.nic.in"},
		{BaseURL: "https://iocletenders.nic.in/nicgep/app", State: "IOCL", Domain: "iocletenders.nic.in"},
		{BaseURL: "https://cpcletenders.nic.in/nicgep/app", State: "CPCL", Domain: "cpcletenders.nic.in"},
		{BaseURL: "https://eprocurebel.co.in/nicgep/app", State: "BEL", Domain: "eprocurebel.co.in"},
		{BaseURL: "https://eprocurentpc.nic.in/nicgep/app", State: "NTPC", Domain: "eprocurentpc.nic.in"},
		{BaseURL: "https://eprocuregsl.nic.in/nicgep/app", State: "GSL", Domain: "eprocuregsl.nic.in"},
		{BaseURL: "https://eprocurehsl.nic.in/nicgep/app", State: "HSL", Domain: "eprocurehsl.nic.in"},
		{BaseURL: "https://eprocuremdl.nic.in/nicgep/app", State: "MDL", Domain: "eprocuremdl.nic.in"},
		{BaseURL: "https://www.eprocuremidhani.nic.in/nicgep/app", State: "Midhani", Domain: "eprocuremidhani.nic.in"},
		{BaseURL: "https://eprocuregrse.co.in/nicgep/app", State: "GRSE", Domain: "eprocuregrse.co.in"},
		{BaseURL: "https://eprocurebhel.co.in/nicgep/app", State: "BHEL", Domain: "eprocurebhel.co.in"},
		{BaseURL: "https://arunachaltenders.gov.in/nicgep/app", State: "ArunachalPradesh", Domain: "arunachaltenders.gov.in"},
		{BaseURL: "https://eprocure.andamannicobar.gov.in/nicgep/app", State: "AndamanNicobar", Domain: "eprocure.andamannicobar.gov.in"},
		{BaseURL: "https://assamtenders.gov.in/nicgep/app", State: "Assam", Domain: "assamtenders.gov.in"},
		{BaseURL: "https://etenders.chd.nic.in/nicgep/app", State: "Chandigarh", Domain: "etenders.chd.nic.in"},
		{BaseURL: "https://dnhtenders.gov.in/nicgep/app", State: "DadarNagarHaveli", Domain: "dnhtenders.gov.in"},
		{BaseURL: "https://ddtenders.gov.in/nicgep/app", State: "DamanDiu", Domain: "ddtenders.gov.in"},
		{BaseURL: "https://govtprocurement.delhi.gov.in/nicgep/app", State: "Delhi", Domain: "govtprocurement.delhi.gov.in"},
		{BaseURL: "https://eprocure.goa.gov.in/nicgep/app", State: "Goa", Domain: "eprocure.goa.gov.in"},
		{BaseURL: "https://etenders.hry.nic.in/nicgep/app", State: "Harayana", Domain: "etenders.hry.nic.in"},
		{BaseURL: "https://hptenders.gov.in/nicgep/app", State: "HimachalPradesh", Domain: "hptenders.gov.in"},
		{BaseURL: "https://jktenders.gov.in/nicgep/app", State: "JammuKashmir", Domain: "jktenders.gov.in"},
		{BaseURL: "https://jharkhandtenders.gov.in/nicgep/app", State: "Jharkhand", Domain: "jharkhandtenders.gov.in"},
		{BaseURL: "https://etenders.kerala.gov.in/nicgep/app", State: "Kerala", Domain: "etenders.kerala.gov.in"},
		{BaseURL: "https://tenders.ladakh.gov.in/nicgep/app", State: "Ladakh", Domain: "tenders.ladakh.gov.in"},
		{BaseURL: "https://tendersutl.gov.in/nicgep/app", State: "Lakshadweep", Domain: "tendersutl.gov.in"},
		{BaseURL: "https://mahatenders.gov.in/nicgep/app", State: "Maharashtra", Domain: "mahatenders.gov.in"},
		{BaseURL: "https://mptenders.gov.in/nicgep/app", State: "MadhyaPradesh", Domain: "mptenders.gov.in"},
		{BaseURL: "https://manipurtenders.gov.in/nicgep/app", State: "Manipur", Domain: "manipurtenders.gov.in"},
		{BaseURL: "https://meghalayatenders.gov.in/nicgep/app", State: "Meghalaya", Domain: "meghalayatenders.gov.in"},
		{BaseURL: "https://mizoramtenders.gov.in/nicgep/app", State: "Mizoram", Domain: "mizoramtenders.gov.in"},
		{BaseURL: "https://nagalandtenders.gov.in/nicgep/app", State: "Nagaland", Domain: "nagalandtenders.gov.in"},
		{BaseURL: "https://tendersodisha.gov.in/nicgep/app", State: "Odisha", Domain: "tendersodisha.gov.in"},
		{BaseURL: "https://pudutenders.gov.in/nicgep/app", State: "Puducherry", Domain: "pudutenders.gov.in"},
		{BaseURL: "https://eproc.punjab.gov.in/nicgep/app", State: "Punjab", Domain: "eproc.punjab.gov.in"},
		{BaseURL: "https://eproc.rajasthan.gov.in/nicgep/app", State: "Rajasthan", Domain: "eproc.rajasthan.gov.in"},
		{BaseURL: "https://sikkimtender.gov.in/nicgep/app", State: "Sikkim", Domain: "sikkimtender.gov.in"},
		{BaseURL: "https://tntenders.gov.in/nicgep/app", State: "TamilNadu", Domain: "tntenders.gov.in"},
		{BaseURL: "https://tripuratenders.gov.in/nicgep/app", State: "Tripura", Domain: "tripuratenders.gov.in"},
		{BaseURL: "https://wbtenders.gov.in/nicgep/app", State: "WestBengal", Domain: "wbtenders.gov.in"},
		{BaseURL: "https://uktenders.gov.in/nicgep/app", State: "Uttarakhand", Domain: "uktenders.gov.in"},
		{BaseURL: "https://etender.up.nic.in/nicgep/app", State: "UttarPradesh", Domain: "etender.up.nic.in"},
	}

	// Reverse the links
	slices.Reverse(baseURLs)

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
