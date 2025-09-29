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
	"github.com/vx6fid/tender-scraper/scraper/pastTenders"
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
	fmt.Println("4.Past Tenders Links")
	fmt.Println("5.Past Tenders Data")
	fmt.Println("6.Document download")
	fmt.Println("7.Prepare FinalLinks.csv")
	var choice int
	fmt.Print("Enter your choice: ")
	fmt.Scan(&choice)

	switch choice {
	case 1:
		runDate := utils.GetRunDate(false)
		// fmt.Println("RunDate: ", runDate)
		linkExtractor := nav.NewLinkExtractor(runDate)
		if err := linkExtractor.Run(); err != nil {
			log.Printf("Link extraction failed: %v", err)
		}
	case 2:
		runDate := utils.GetRunDate(false)
		// fmt.Println("RunDate: ", runDate)
		for _, u := range utils.BaseURLs {
			log.Printf("--- Starting concurrent tender extraction for [%s] ---", u.State)

			// Determine optimal worker count based on expected load
			totalJobs, err := utils.EstimateJobCount(u.State, runDate, false, "")
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
		runDate := utils.GetRunDate(false)
		// fmt.Println("RunDate: ", runDate)
		linkExtractor := nav.NewLinkExtractor(runDate)
		linkExtractor.Corrigendums()
	case 4:
		// Case 4 and 5 are not yet parallel and write safe
		runDate := utils.GetRunDate(false)
		// fmt.Println("RunDate: ", runDate)
		fromStr := "01/01/2025"
		toStr := "30/09/2025"

		tenderType := utils.GiveStageName()

		linkExtractor := nav.NewLinkExtractor(runDate)
		linkExtractor.PastTenders(fromStr, toStr, 7, tenderType)
	case 5:
		runDate := utils.GetRunDate(true)
		dir := fmt.Sprintf("TenderData/PastLinks/%s", runDate)

		tenderType := utils.GiveStageName()
		err := pastTenders.Run(dir, runDate, tenderType)
		if err != nil {
			log.Printf("Error running past tenders: %v", err)
		}
	case 6:
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
				Name: "p3.pdf",
				Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_9\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=SkAKgGSlUfrrabyHogXFP%2BF9cVzHVh8EwOS0Ljqv9%2BBY%3D",
				Type: "Date",
			},
			{
				Name: "215916Corrigendum-IX.pdf",
				Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_9\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=SPt7iTDx7TtDUZNUJ7t7j8qwjbxs1fPtg%2F6dojOX8LFxPJz10Q77j0LOlUHCEsqJp",
				Type: "Date",
			},
			{
				Name: "3.pdf",
				Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_9\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=S7QhBDSRHSnT0S%2BSIC1HC2BvjtJQlToGHFpYVVNsa%2F18%3D",
				Type: "Date",
			},
			{
				Name: "CRRRP1.pdf",
				Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_9\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=SCC4SOtfWYPeLJWQnsyNHKrPqspqYEYzPFutHtIQMI8k%3D",
				Type: "Date",
			},
			{
				Name: "215916Corrigendum-VI.pdf",
				Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_9\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=SyvwozV5Tyf%2FurSh7Th3zgjTofb0ZYrrKxS%2BOLmBsa1J6nVVCRt7A5iB9%2FKXWZ1w%2B",
				Type: "Date",
			},
			{
				Name: "Corrigendum-V-Pkg-III.pdf",
				Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_9\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=SYqtHDanuMRaWd%2F7%2FtVwZJvYeKSv3x2okXzIAOUROet6p9WmK6qt71Crd3EYtffTm",
				Type: "Date",
			},
			{
				Name: "215916Corrigendum-IV.pdf",
				Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_9\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=S8g9R57veRQ%2BNIHwEotutPnRwm54HRvl8movENWaxnSF36kMPdq%2FYcBqoZHm%2FHR9o",
				Type: "Date",
			},
			{
				Name: "Corri_005.pdf",
				Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_9\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=Se5D4ma9Rr3b5bnYWd0Fca0zzTixTEl1nNFg32pKSA%2FI%3D",
				Type: "Date",
			},
			{
				Name: "II.pdf",
				Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_9\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=S2Im7IDUD9LleiuxE9RNVG%2FYpJpvW9FdG1eIv8CWknO8%3D",
				Type: "Date",
			},
			{
				Name: "286A004.pdf",
				Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_9\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=SOhU6dIVMSI0D5S1mZbUl8QIb8lq71JfQ%2BmDjNG3QG9E%3D",
				Type: "Date",
			},
			{
				Name: "7020.pdf",
				Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_41\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=Sacb93OC9%2B9Kl%2FrMQHu6rDlCS8qgPX7ttE5%2BAX71JLh0%3D",
				Type: "Others",
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
			{
				Name: "Pkg-III-DOCS.part04.rar",
				Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_41\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=SE4py8nYjgELtwyHSJrlfZf6vpStTnyePpCdmf1CSoHH3cXw677CljkHKSnrtVQf8",
				Type: "Others",
			},
			{
				Name: "Pkg-III-DOCS.part03.rar",
				Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_41\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=SvGWCH%2Bbd6CRvPbp%2BwP73ezb%2BPnFF%2F%2FjsHgWvPutUwTRgjHD6SBgdkjBx8Goro1c%2F",
				Type: "Others",
			},
			{
				Name: "Pkg-III-DOCS.part02.rar",
				Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_41\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=S6Tv9OsVplbZ88xpVXKww%2BErfxa3VHEzI8jF2UugRsGavFH0TWX010Kb9o6W1z8KE",
				Type: "Others",
			},
			{
				Name: "Pkg-III-DOCS.part01.rar",
				Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_41\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=SigxWIAJ2IV36VZSOi5K%2FR12%2Besh%2FiyVOUVIyK%2B6s7SsiXMpZX4X0UfQ7LKBA0%2Fet",
				Type: "Others",
			},
		}

		// Check if the tender folder exists in the S3 bucket
		if exists, err := utils.CheckTenderFolderExists("tenderbharat", _id); err != nil {
			log.Fatalf("failed to check if tender folder exists: %v", err)
		} else if exists {
			log.Printf("Tender folder already exists for %s", _id)
			return
		}

		fmt.Printf("Starting Tender Docs Download for %s", _id)

		// Replace \u0026 in the corrigendum links
		for i := range corrigendumLinks {
			corrigendumLinks[i].Link = strings.ReplaceAll(corrigendumLinks[i].Link, "\\u0026", "&")
			// fmt.Println(corrigendumLinks[i].Link)
		}

		// Replace \u0026 in the tenderURL
		tenderURL = strings.ReplaceAll(tenderURL, "\\u0026", "&")

		baseURL, state, err := utils.GetBaseURLAndState(tenderURL)
		if err != nil {
			fmt.Println("Error:", err)
		} else {
			fmt.Printf("BaseURL: %s, State: %s\n", baseURL, state)
		}

		// 1. Create session
		sess := session.NewSession(baseURL, state)

		// 2. Establish tender session (solves tender captcha, stores cookies)
		if err := sess.EstablishSession("ActiveTenders"); err != nil {
			log.Fatalf("[%s] failed to establish tender session: %v", state, err)
		}

		// 3. Create doc downloader with that session
		downloader := docdownload.NewDocDownloader(sess, state, log.Default())

		// 4. For each tender URL you want, run the doc download flow
		if err := downloader.Run(_id, tenderURL, corrigendumLinks); err != nil {
			log.Printf("[%s] doc download failed: %v", state, err)
		}
		// Get results
		nitDocs, ZipFiles := downloader.GetResults()
		log.Printf("Extracted %d NIT documents and %s zip files", len(nitDocs), ZipFiles.DocumentName)

		// Clean up for reuse
		downloader.Reset()
	case 7:
		runDate := utils.GetRunDate(false)
		for _, u := range utils.BaseURLs {
			err := utils.FinalCSV(runDate, u.State)
			if err != nil {
				fmt.Printf("%v\n", err)
			} else {
				fmt.Printf("[%s] CSV file generated successfully\n", u.State)
			}
		}
	case 8:
		for _, u := range utils.BaseURLs {
			domain, err := utils.GetDomain(u.BaseURL)
			if err != nil {
				fmt.Printf("%v\n", err)
			} else {
				fmt.Printf("%s\n", domain)
			}
		}
	default:
		fmt.Println("Invalid choice")
		os.Exit(1)
	}

	log.Println("Scraping completed successfully")
}
