package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/vx6fid/tender-scraper/cli/commands"
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

func runCorrigendumLinks() {
	if err := commands.ExtractCorrigendumLinks(log.Default()); err != nil {
		log.Printf("Corrigendum links extraction failed: %v", err)
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

func runFinalCSV() {
	if err := commands.PrepareFinalCSV(log.Default()); err != nil {
		log.Printf("Final CSV generation failed: %v", err)
	}
}

func runCountTotalLinks() {
	if err := commands.CountTotalLinks(); err != nil {
		log.Printf("Total links count failed: %v", err)
	}
}
