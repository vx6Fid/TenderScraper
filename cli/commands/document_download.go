package commands

import (
	"fmt"
	"log"
	"strings"

	docdownload "github.com/vx6fid/tender-scraper/docDownloads"
	"github.com/vx6fid/tender-scraper/session"
	"github.com/vx6fid/tender-scraper/utils"
)

func DownloadDocuments(logger *log.Logger) error {
	config := getDocumentDownloadConfig()

	if exists, err := utils.CheckTenderFolderExists("tenderbharat", config.ID); err != nil {
		return fmt.Errorf("failed to check if tender folder exists: %w", err)
	} else if exists {
		logger.Printf("Tender folder already exists for %s", config.ID)
		return nil
	}

	logger.Printf("Starting Tender Docs Download for %s", config.ID)

	normalizeLinks(&config)

	baseURL, state, err := utils.GetBaseURLAndState(config.TenderURL)
	if err != nil {
		return fmt.Errorf("failed to get base URL and state: %w", err)
	}

	logger.Printf("BaseURL: %s, State: %s\n", baseURL, state)

	sess := session.NewSession(baseURL, state)
	if err := sess.EstablishSession("ActiveTenders"); err != nil {
		return fmt.Errorf("[%s] failed to establish tender session: %w", state, err)
	}

	downloader := docdownload.NewDocDownloader(sess, state, logger)
	if err := downloader.Run(config.ID, config.TenderURL, config.CorrigendumLinks); err != nil {
		return fmt.Errorf("[%s] doc download failed: %w", state, err)
	}

	nitDocs, zipFiles := downloader.GetResults()
	logger.Printf("Extracted %d NIT documents and %s zip files", len(nitDocs), zipFiles.DocumentName)

	downloader.Reset()
	return nil
}

type DocumentConfig struct {
	ID               string
	TenderURL        string
	CorrigendumLinks []utils.CorrLinks
}

func getDocumentDownloadConfig() DocumentConfig {
	return DocumentConfig{
		ID:        "68695b2955d119428e5086ab",
		TenderURL: "https://etenders.gov.in/eprocure/app?component=%24DirectLink&page=FrontEndLatestActiveCorrigendums&service=direct&session=T&sp=SwKgOCf7CLcX8A0VIcO%2FJUA%3D%3D",
		CorrigendumLinks: []utils.CorrLinks{
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
				Name: "7020.pdf",
				Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_41\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=Sacb93OC9%2B9Kl%2FrMQHu6rDlCS8qgPX7ttE5%2BAX71JLh0%3D",
				Type: "Others",
			},
			{
				Name: "Pkg-III-DOCS.part06.rar",
				Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_41\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=SYBgqgF2rfPSuzM%2F4m3729eUcv4IY4eKw825%2BKxXulQmrRv1rpDQuT0%2F%2Bft9JQhCI",
				Type: "Others",
			},
		},
	}
}

func normalizeLinks(config *DocumentConfig) {
	for i := range config.CorrigendumLinks {
		config.CorrigendumLinks[i].Link = strings.ReplaceAll(config.CorrigendumLinks[i].Link, "\\u0026", "&")
	}
	config.TenderURL = strings.ReplaceAll(config.TenderURL, "\\u0026", "&")
}
