package main

import (
	"fmt"
	"os"

	"github.com/vx6fid/tender-scraper/scraper"
)

func main() {
	// Base URLs
	urls := []string{
		"https://eproc.rajasthan.gov.in/nicgep/app",
	}

	fmt.Println("--- Tender Scraper Initialized ---")
	fmt.Println("Choose Option:")
	fmt.Println("1. Search Tenders")
	fmt.Println("2. Active Tenders")
	fmt.Println("3. Corrigendum")
	fmt.Println("4. Exit")

	fmt.Println("Enter choice:")
	var choice int
	fmt.Scanln(&choice)

	if choice == 4 {
		fmt.Println("Exiting...")
		os.Exit(0)
	} else if choice < 1 || choice > 3 {
		fmt.Println("Invalid choice")
		os.Exit(1)
	}

	for _, url := range urls {
		scraper.ScrapeTenders(url, choice)
	}
}
