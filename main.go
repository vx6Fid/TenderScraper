package main

import (
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/joho/godotenv"
	docdownload "github.com/vx6fid/tender-scraper/docDownloads"
	"github.com/vx6fid/tender-scraper/scraper/extract"
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
		// {BaseURL: "https://eprocure.gov.in/eprocure/app", State: "eProcurementCentralGovernment", Domain: "eprocure.gov.in"}, // eprocure only for this
		// {BaseURL: "https://defproc.gov.in/nicgep/app", State: "MinistryOfDefence", Domain: "defproc.gov.in"},
		// {BaseURL: "https://pmgsytenders.gov.in/nicgep/app", State: "PMGSY", Domain: "pmgsytenders.gov.in"},
		{BaseURL: "https://etenders.gov.in/eprocure/app", State: "PSU", Domain: "etenders.gov.in"},
		// {BaseURL: "https://coalindiatenders.nic.in/nicgep/app", State: "CoalIndia", Domain: "coalindiatenders.nic.in"},
		// {BaseURL: "https://iocletenders.nic.in/nicgep/app", State: "IOCL", Domain: "iocletenders.nic.in"},
		// {BaseURL: "https://cpcletenders.nic.in/nicgep/app", State: "CPCL", Domain: "cpcletenders.nic.in"},
		// {BaseURL: "https://eprocurebel.co.in/nicgep/app", State: "BEL", Domain: "eprocurebel.co.in"},
		// {BaseURL: "https://eprocurentpc.nic.in/nicgep/app", State: "NTPC", Domain: "eprocurentpc.nic.in"},
		// {BaseURL: "https://eprocuregsl.nic.in/nicgep/app", State: "GSL", Domain: "eprocuregsl.nic.in"},
		// {BaseURL: "https://eprocurehsl.nic.in/nicgep/app", State: "HSL", Domain: "eprocurehsl.nic.in"},
		// {BaseURL: "https://eprocuremdl.nic.in/nicgep/app", State: "MDL", Domain: "eprocuremdl.nic.in"},
		// {BaseURL: "https://www.eprocuremidhani.nic.in/nicgep/app", State: "Midhani", Domain: "eprocuremidhani.nic.in"},
		// {BaseURL: "https://eprocuregrse.co.in/nicgep/app", State: "GRSE", Domain: "eprocuregrse.co.in"},
		// {BaseURL: "https://eprocurebhel.co.in/nicgep/app", State: "BHEL", Domain: "eprocurebhel.co.in"},
		// {BaseURL: "https://arunachaltenders.gov.in/nicgep/app", State: "ArunachalPradesh", Domain: "arunachaltenders.gov.in"},
		// {BaseURL: "https://eprocure.andamannicobar.gov.in/nicgep/app", State: "AndamanNicobar", Domain: "eprocure.andamannicobar.gov.in"},
		// {BaseURL: "https://assamtenders.gov.in/nicgep/app", State: "Assam", Domain: "assamtenders.gov.in"},
		// {BaseURL: "https://etenders.chd.nic.in/nicgep/app", State: "Chandigarh", Domain: "etenders.chd.nic.in"},
		// {BaseURL: "https://dnhtenders.gov.in/nicgep/app", State: "DadarNagarHaveli", Domain: "dnhtenders.gov.in"},
		// {BaseURL: "https://ddtenders.gov.in/nicgep/app", State: "DamanDiu", Domain: "ddtenders.gov.in"},
		// {BaseURL: "https://govtprocurement.delhi.gov.in/nicgep/app", State: "Delhi", Domain: "govtprocurement.delhi.gov.in"},
		// {BaseURL: "https://eprocure.goa.gov.in/nicgep/app", State: "Goa", Domain: "eprocure.goa.gov.in"},
		// {BaseURL: "https://etenders.hry.nic.in/nicgep/app", State: "Harayana", Domain: "etenders.hry.nic.in"},
		// {BaseURL: "https://hptenders.gov.in/nicgep/app", State: "HimachalPradesh", Domain: "hptenders.gov.in"},
		// {BaseURL: "https://jktenders.gov.in/nicgep/app", State: "JammuKashmir", Domain: "jktenders.gov.in"},
		// {BaseURL: "https://jharkhandtenders.gov.in/nicgep/app", State: "Jharkhand", Domain: "jharkhandtenders.gov.in"},
		// {BaseURL: "https://etenders.kerala.gov.in/nicgep/app", State: "Kerala", Domain: "etenders.kerala.gov.in"},
		// {BaseURL: "https://tenders.ladakh.gov.in/nicgep/app", State: "Ladakh", Domain: "tenders.ladakh.gov.in"},
		// {BaseURL: "https://tendersutl.gov.in/nicgep/app", State: "Lakshadweep", Domain: "tendersutl.gov.in"},
		// {BaseURL: "https://mahatenders.gov.in/nicgep/app", State: "Maharashtra", Domain: "mahatenders.gov.in"},
		// {BaseURL: "https://mptenders.gov.in/nicgep/app", State: "MadhyaPradesh", Domain: "mptenders.gov.in"},
		// {BaseURL: "https://manipurtenders.gov.in/nicgep/app", State: "Manipur", Domain: "manipurtenders.gov.in"},
		// {BaseURL: "https://meghalayatenders.gov.in/nicgep/app", State: "Meghalaya", Domain: "meghalayatenders.gov.in"},
		// {BaseURL: "https://mizoramtenders.gov.in/nicgep/app", State: "Mizoram", Domain: "mizoramtenders.gov.in"},
		// {BaseURL: "https://nagalandtenders.gov.in/nicgep/app", State: "Nagaland", Domain: "nagalandtenders.gov.in"},
		// {BaseURL: "https://tendersodisha.gov.in/nicgep/app", State: "Odisha", Domain: "tendersodisha.gov.in"},
		// {BaseURL: "https://pudutenders.gov.in/nicgep/app", State: "Puducherry", Domain: "pudutenders.gov.in"},
		// {BaseURL: "https://eproc.punjab.gov.in/nicgep/app", State: "Punjab", Domain: "eproc.punjab.gov.in"},
		// {BaseURL: "https://eproc.rajasthan.gov.in/nicgep/app", State: "Rajasthan", Domain: "eproc.rajasthan.gov.in"},
		// {BaseURL: "https://sikkimtender.gov.in/nicgep/app", State: "Sikkim", Domain: "sikkimtender.gov.in"},
		// {BaseURL: "https://tntenders.gov.in/nicgep/app", State: "TamilNadu", Domain: "tntenders.gov.in"},
		// {BaseURL: "https://tripuratenders.gov.in/nicgep/app", State: "Tripura", Domain: "tripuratenders.gov.in"},
		// {BaseURL: "https://wbtenders.gov.in/nicgep/app", State: "WestBengal", Domain: "wbtenders.gov.in"},
		// {BaseURL: "https://uktenders.gov.in/nicgep/app", State: "Uttarakhand", Domain: "uktenders.gov.in"},
		// {BaseURL: "https://etender.up.nic.in/nicgep/app", State: "UttarPradesh", Domain: "etender.up.nic.in"},
	}

	// Reverse the links
	slices.Reverse(baseURLs)

	fmt.Println("--- Choose one of the following ---")
	fmt.Println("1.Tender links")
	fmt.Println("2.Tender data")
	fmt.Println("3.Corrigendum Links")
	fmt.Println("4.Document download")
	var choice int
	fmt.Print("Enter your choice: ")
	fmt.Scan(&choice)

	switch choice {
	case 1:
		runDate := utils.GetRunDate()
		// fmt.Println("RunDate: ", runDate)
		linkExtractor := nav.NewLinkExtractor(runDate, baseURLs)
		if err := linkExtractor.Run(); err != nil {
			log.Printf("Link extraction failed: %v", err)
		}
	case 2:
		runDate := utils.GetRunDate()
		// fmt.Println("RunDate: ", runDate)
		for _, u := range baseURLs {
			log.Printf("--- Starting concurrent tender extraction for [%s] ---", u.State)

			// Determine optimal worker count based on expected load
			totalJobs, err := utils.EstimateJobCount(u.State, runDate)
			if err != nil {
				log.Printf("[%s] failed to estimate job count: %v", u.State, err)
				continue
			}

			optimalWorkers := utils.CalculateOptimalWorkers(totalJobs)

			extractor := extract.NewConcurrentExtractor(u.BaseURL, u.Domain, u.State, runDate, optimalWorkers)

			if err := extractor.ExtractTendersWithMultipleSessions(); err != nil {
				log.Printf("[%s] concurrent extraction failed: %v", u.State, err)
				continue
			}

			log.Printf("--- Completed [%s] ---", u.State)
		}
	case 3:
		runDate := utils.GetRunDate()
		// fmt.Println("RunDate: ", runDate)
		linkExtractor := nav.NewLinkExtractor(runDate, baseURLs)
		linkExtractor.Corrigendums()
	case 4:
		baseURL := "https://etenders.gov.in/eprocure/app"
		state := "PSU"

		// 1. Create session
		sess := session.NewSession(baseURL, state)

		// 2. Establish tender session (solves tender captcha, stores cookies)
		if err := sess.EstablishSession("ActiveTenders"); err != nil {
			log.Fatalf("[%s] failed to establish tender session: %v", state, err)
		}

		// 3. Create doc downloader with that session
		downloader := docdownload.NewDocDownloader(sess, state, log.Default())

		// 4. For each tender URL you want, run the doc download flow
		_id := "68695b2955d119428e5086ab"
		tenderURL := "https://etenders.gov.in/eprocure/app?component=%24DirectLink&page=FrontEndLatestActiveCorrigendums&service=direct&session=T&sp=SwKgOCf7CLcX8A0VIcO%2FJUA%3D%3D"
		corrigendumLinks := []utils.CorrLinks{
			{
				Name: "267020.pdf",
				Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_9\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=SXa2%2Bv1L%2FiUoZA9%2FQE0KbZajv0bVL9ByrEDT3Hk8pjFA%3D",
				Type: "Date",
			},
			{
				Name: "C3.pdf",
				Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_9\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=SCFSvWn251d7lvuC4Nh5dGcW686t3AHcBGQneN2DULys%3D",
				Type: "Date",
			},
			{
				Name: "Pkg-III-DOCS.part06.rar",
				Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_41\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=SYBgqgF2rfPSuzM%2F4m3729eUcv4IY4eKw825%2BKxXulQmrRv1rpDQuT0%2F%2Bft9JQhCI",
				Type: "Others",
			},
			{
				Name: "Pkg-III-DOCS.part05.rar",
				Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_41\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=Se3MER5jkdgXexbF0bo2OQcNNDmAMce5eRz8025QjC5foMCIGr0bNcBoo3cWWc4qs",
				Type: "Others",
			},
		}

		// Replace \u0026 in the corrigendum links
		for i := range corrigendumLinks {
			corrigendumLinks[i].Link = strings.ReplaceAll(corrigendumLinks[i].Link, "\\u0026", "&")
			fmt.Println(corrigendumLinks[i].Link)
		}

		// break

		if err := downloader.Run(_id, tenderURL, corrigendumLinks); err != nil {
			log.Printf("[%s] doc download failed: %v", state, err)
		}
		// Get results
		nitDocs, ZipFiles := downloader.GetResults()
		log.Printf("Extracted %d NIT documents and %s zip files", len(nitDocs), ZipFiles.DocumentName)

		// Clean up for reuse
		downloader.Reset()
	default:
		fmt.Println("Invalid choice")
		os.Exit(1)
	}

	log.Println("Scraping completed successfully")
}
