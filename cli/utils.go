package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/vx6fid/tender-scraper/cli/commands"
	"github.com/vx6fid/tender-scraper/utils"
)

var Logger = log.New(os.Stdout, "[TenderScraper] ", log.LstdFlags)

// LoadEnvOrFatal loads environment variables from a .env file.
func LoadEnvOrFatal() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
}

//-----------------------------------
// Handlers to Run Commmands
//-----------------------------------

// Wrappers for commands to handle error logging
func runTenderLinks() {
	if err := commands.ExtractTenderLinks(log.Default()); err != nil {
		log.Printf("Tender links extraction failed: %v", err)
	}
}

func runTenderData() {
	if err := commands.ExtractTenderData(log.Default()); err != nil {
		log.Printf("Tender data extraction failed: %v", err)
	}
}

func runPastTenderLinks() {
	if err := commands.ExtractPastTenderLinks(log.Default()); err != nil {
		log.Printf("Past tender links extraction failed: %v", err)
	}
}

func runPastTenderData() {
	if err := commands.ExtractPastTenderData(log.Default()); err != nil {
		log.Printf("Past tender data extraction failed: %v", err)
	}
}

func runDocDownload() {
	if err := commands.DownloadDocuments(log.Default()); err != nil {
		log.Printf("Document download failed: %v", err)
	}
}

func runPrintSearchTendersURL() {
	for _, u := range utils.BaseURLs {
		url := utils.BuildPageURLRaw(u.BaseURL, 2)
		fmt.Println(url)
	}
}
