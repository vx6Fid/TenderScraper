package scraper

// ScrapeTenders dispatches to the relevant scraping function
// func ScrapeTenders(baseURL string, choice int) {
// 	u, err := url.Parse(baseURL)
// 	if err != nil {
// 		fmt.Printf("Error parsing URL: %v\n", err)
// 		return
// 	}

// 	c := NewCollector(u.Host)

// 	switch choice {
// 	case 1:
// 		nav.ScrapeSearch(c, baseURL)
// 	case 2:
// 		fmt.Println("Scraping active tenders...")
// 	case 3:
// 		nav.ScrapeCorrigendum(c, baseURL)
// 	default:
// 		fmt.Println("Invalid choice. Please select 1, 2, or 3.")
// 		return
// 	}
// }
