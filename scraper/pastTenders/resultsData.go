package pastTenders

import (
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	types "github.com/vx6fid/tender-scraper/utils/types"
)

func (pt *PastTender) setupPastTenderDataHandler(c *colly.Collector, pastTenderData *PastTendersData, URL string) {
	c.OnHTML("html", func(e *colly.HTMLElement) {
		doc := e.DOM

		// log.Println("[SCRAPER] Starting to extract tender data...")

		// Extract all sections
		pt.extractBidsList(doc, pastTenderData)
		pt.extractStageUpdates(doc, pastTenderData)
		pt.extractFinancialEvaluationBids(doc, pastTenderData)
		pt.extractAwardedBids(doc, pastTenderData)

		// log.Printf("[SCRAPER] Extraction complete. Found: %d bids, %d financial evaluations, %d awarded bids",
		// len(pastTenderData.Bids),
		// len(pastTenderData.FinancialEvaluationBidList),
		// len(pastTenderData.AwardedBidsList))
	})

	if err := c.Visit(URL); err != nil {
		// log.Printf("[Navigation] Failed to visit %s: %v", URL, err)
	}
}

func (pt *PastTender) extractBidsList(doc *goquery.Selection, pastTenderData *PastTendersData) {
	// log.Println("[SCRAPER] Extracting bids list...")

	doc.Find("#bidValidTableView tr.td_field").Each(func(i int, s *goquery.Selection) {
		bid := types.Bid{}

		s.Find("td").Each(func(j int, td *goquery.Selection) {
			text := strings.TrimSpace(td.Text())
			switch j {
			case 0: // S.No
				if sno, err := strconv.Atoi(text); err == nil {
					bid.SNo = sno
				}
			case 1: // Bid Number
				if link := td.Find("a"); link.Length() > 0 {
					bid.BidNumber = strings.TrimSpace(link.Text())
				} else {
					bid.BidNumber = text
				}
			case 2: // Bidder Name
				bid.BidderName = text
			case 3: // Submitted Date
				bid.SubmittedDate = strings.TrimSpace(text)
			case 4: // Status
				bid.Status = text
			case 5: // Remarks
				bid.Remarks = text
			case 6: // Status Updated On
				bid.StatusUpdatedOn = strings.TrimSpace(text)
			}
		})

		if bid.BidNumber != "" {
			// log.Printf("[SCRAPER] Found bid: %s - %s - %s", bid.BidNumber, bid.BidderName, bid.Status)
			pastTenderData.Bids = append(pastTenderData.Bids, bid)
		}
	})

	// log.Printf("[SCRAPER] Extracted %d bids from main table", len(pastTenderData.Bids))
}

func (pt *PastTender) extractStageUpdates(doc *goquery.Selection, pastTenderData *PastTendersData) {
	// // log.Println("[SCRAPER] Extracting stage updates...")

	doc.Find("table").Each(func(i int, table *goquery.Selection) {
		sectionHeader := strings.TrimSpace(table.Find(".section_head").Text())
		// // log.Printf("[SCRAPER] Processing table with header: '%s'", sectionHeader)

		switch {
		case strings.Contains(sectionHeader, "Bid Opening Summary"):
			pt.extractBidOpeningSummary(table, pastTenderData)

		case strings.Contains(sectionHeader, "Technical Evaluation Summary Details"):
			pt.extractTechnicalEvaluationSummary(table, pastTenderData)

		case strings.Contains(sectionHeader, "Finance Evaluation Summary Details"):
			pt.extractFinanceEvaluationSummary(table, pastTenderData)

		case strings.Contains(sectionHeader, "AOC"):
			pt.extractAOCSummary(table, pastTenderData)
		}
	})
}

func (pt *PastTender) extractBidOpeningSummary(table *goquery.Selection, pastTenderData *PastTendersData) {
	// // log.Println("[SCRAPER] Extracting bid opening summary...")

	table.Find("tr").Each(func(j int, row *goquery.Selection) {
		firstCell := strings.TrimSpace(row.Find("td").First().Text())
		if strings.Contains(firstCell, "Updated On") {
			dateText := cleanText(row.Find("td").Last().Text())
			pastTenderData.StageUpdates.TechnicalBidOpeningUpdatedOn = strings.TrimSpace(dateText)
			// // log.Printf("[SCRAPER] Bid opening updated on: %s", dateText)
		}
	})
}

func (pt *PastTender) extractTechnicalEvaluationSummary(table *goquery.Selection, pastTenderData *PastTendersData) {
	// // log.Println("[SCRAPER] Extracting technical evaluation summary...")

	table.Find("tr").Each(func(j int, row *goquery.Selection) {
		firstCell := strings.TrimSpace(row.Find("td").First().Text())
		if strings.Contains(firstCell, "Updated On") {
			dateText := cleanText(row.Find("td").Last().Text())
			pastTenderData.StageUpdates.TechnicalEvaluationUpdatedOn = strings.TrimSpace(dateText)
			// // log.Printf("[SCRAPER] Technical evaluation updated on: %s", dateText)
		}
	})
}

func (pt *PastTender) extractFinanceEvaluationSummary(table *goquery.Selection, pastTenderData *PastTendersData) {
	// // log.Println("[SCRAPER] Extracting finance evaluation summary...")

	table.Find("tr").Each(func(j int, row *goquery.Selection) {
		firstCell := strings.TrimSpace(row.Find("td").First().Text())
		if strings.Contains(firstCell, "Updated on") {
			dateText := cleanText(row.Find("td").Last().Text())
			pastTenderData.StageUpdates.FinancialEvaluationUpdatedOn = strings.TrimSpace(dateText)
			// // log.Printf("[SCRAPER] Finance evaluation updated on: %s", dateText)
		}
	})
}

func (pt *PastTender) extractAOCSummary(table *goquery.Selection, pastTenderData *PastTendersData) {
	// // log.Println("[SCRAPER] Extracting AOC summary...")

	table.Find("tr").Each(func(j int, row *goquery.Selection) {
		firstCell := strings.TrimSpace(row.Find("td").First().Text())
		if strings.Contains(firstCell, "Updated on") {
			dateText := cleanText(row.Find("td").Last().Text())
			pastTenderData.StageUpdates.AOCUpdatedOn = strings.TrimSpace(dateText)
			// // log.Printf("[SCRAPER] AOC updated on: %s", dateText)
		}
		if strings.Contains(firstCell, "Contract Value") {
			contractValue := cleanText(row.Find("td").Last().Text())
			pastTenderData.ContractValue = parseAmountToFloat(strings.TrimSpace(contractValue))
			// // log.Printf("[SCRAPER] Contract value: %s", contractValue)
		}
	})
}

func (pt *PastTender) extractFinancialEvaluationBids(sel *goquery.Selection, pastTenderData *PastTendersData) {
	// log.Println("[SCRAPER] Extracting financial evaluation bids...")

	// Use more specific selector to find the exact table with id="table_list"
	table := sel.Find("table#table_list").First()
	if table.Length() == 0 {
		// log.Println("[SCRAPER] No financial evaluation table found with id 'table_list'")
		return
	}

	sectionHeader := strings.TrimSpace(table.Find(".section_head").Text())
	// log.Printf("[SCRAPER] Found financial evaluation table: %s", sectionHeader)

	if strings.Contains(sectionHeader, "Financial Evaluation Bid List") {
		table.Find("tr[id^='informal']").Each(func(j int, row *goquery.Selection) {
			financial := types.FinancialEvaluationBidList{}

			row.Find("td").Each(func(k int, td *goquery.Selection) {
				text := strings.TrimSpace(td.Text())
				switch k {
				case 1: // Bid Number
					financial.BidNumber = text
				case 2: // Bidder Name
					financial.BidderName = text
				case 3: // Value
					financial.Value = parseAmountToFloat(text)
				case 4: // Rank
					financial.Rank = text
				}
			})

			if financial.BidNumber != "" {
				// log.Printf("[SCRAPER] Financial evaluation: %s - %s - %.2f - %s",
				// 	financial.BidNumber, financial.BidderName, financial.Value, financial.Rank)
				pastTenderData.FinancialEvaluationBidList = append(pastTenderData.FinancialEvaluationBidList, financial)
			}
		})
	}

	// log.Printf("[SCRAPER] Extracted %d financial evaluation bids", len(pastTenderData.FinancialEvaluationBidList))
}

func (pt *PastTender) extractAwardedBids(doc *goquery.Selection, pastTenderData *PastTendersData) {
	// log.Println("[SCRAPER] Extracting awarded bids...")

	doc.Find("#bidAocTableView tr[id^='informal']").Each(func(i int, row *goquery.Selection) {
		awarded := types.AwardedBidsList{}

		row.Find("td").Each(func(j int, td *goquery.Selection) {
			text := strings.TrimSpace(td.Text())
			switch j {
			case 1: // Bid Number
				awarded.BidNumber = text
			case 2: // Bidder Name
				awarded.BidderName = text
			case 3: // Awarded Currency
				awarded.AwardedCurrency = text
			case 4: // Awarded Value
				awarded.AwardedValue = parseAmountToFloat(text)
			}
		})

		if awarded.BidNumber != "" {
			// log.Printf("[SCRAPER] Awarded bid: %s - %s - %s %.2f",awarded.BidNumber, awarded.BidderName, awarded.AwardedCurrency, awarded.AwardedValue)
			pastTenderData.AwardedBidsList = append(pastTenderData.AwardedBidsList, awarded)
		}
	})

	// log.Printf("[SCRAPER] Extracted %d awarded bids", len(pastTenderData.AwardedBidsList))
}

// Helper function to clean text (remove HTML tags and extra whitespace)
func cleanText(text string) string {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "<b>", "")
	text = strings.ReplaceAll(text, "</b>", "")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	return strings.TrimSpace(text)
}
