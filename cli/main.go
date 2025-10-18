package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	fmt.Print("--- Tender Scraper ---\n")
	LoadEnvOrFatal()

	choice := cliPrompt()
	dispatcher(choice)

	fmt.Println("Scraping completed successfully")
}

func cliPrompt() int {
	options := []string{
		"Tender links",
		"Corr Links",
		"Prepare FinalLinks.csv",
		"Tender data",
		"Past Tenders Links",
		"Past Tenders Data",
		"Document download",
		"Count Number of Links",
		"Print Search Tenders URL",
	}

	for {
		fmt.Println("\nSelect an option:")
		for i, opt := range options {
			fmt.Printf("  %d. %s\n", i+1, opt)
		}
		fmt.Print("> ")

		var choice int
		if _, err := fmt.Scan(&choice); err != nil || choice < 1 {
			fmt.Println("Invalid input, please enter a number greater than 1.")
			continue
		}
		return choice
	}
}

var handlers = map[int]func(){
	1: runTenderLinks,
	2: runCorrigendumLinks,
	3: runFinalCSV,
	4: runTenderData,
	5: runPastTenderLinks,
	6: runPastTenderData,
	7: runDocDownload,
	8: runCountTotalLinks,
	9: runPrintSearchTendersURL,
}

func dispatcher(choice int) {
	if handler, ok := handlers[choice]; ok {
		handler()
	} else {
		fmt.Println("\n[✦] Exiting the scraper....")
		time.Sleep(800 * time.Millisecond)
		os.Exit(0)
	}
}
