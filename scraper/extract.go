package scraper

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/vx6fid/tender-scraper/session"
	"github.com/vx6fid/tender-scraper/utils"
)

type TenderDataScraper struct {
	collector        *colly.Collector
	activeTendersURL string
}

func NewTenderDataScraper(sess *session.Session, domain string, state string) *TenderDataScraper {
	collector := sess.NewCollector(domain)
	// collector.AllowURLRevisit = true
	return &TenderDataScraper{
		collector:        collector,
		activeTendersURL: sess.ActiveTendersURL,
	}
}

func (ts *TenderDataScraper) ExtractTenderData() error {
	log.Println("Starting tender scraping process with correct session flow.")

	inputPath := filepath.Join("TenderLinks", "UttarPradeshLinks.csv")
	inFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open links CSV at %s: %w", inputPath, err)
	}
	defer inFile.Close()

	reader := csv.NewReader(inFile)
	rows, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read links CSV: %w", err)
	}
	if len(rows) <= 1 {
		return fmt.Errorf("no data rows found in %s", inputPath)
	}

	outFile, err := os.Create("tenders.csv")
	if err != nil {
		return fmt.Errorf("failed to create output CSV: %w", err)
	}
	defer func() { _ = outFile.Close() }()

	writer := csv.NewWriter(outFile)
	defer writer.Flush()

	// Prepare structured CSV outputs under out/
	if err := os.MkdirAll("out", 0755); err != nil {
		return fmt.Errorf("failed to create out directory: %w", err)
	}

	basicFile, err := os.Create(filepath.Join("out", "basic_details.csv"))
	if err != nil {
		return fmt.Errorf("failed to create basic_details.csv: %w", err)
	}
	defer func() { _ = basicFile.Close() }()
	basicW := csv.NewWriter(basicFile)
	defer basicW.Flush()

	payFile, err := os.Create(filepath.Join("out", "payment_instruments.csv"))
	if err != nil {
		return fmt.Errorf("failed to create payment_instruments.csv: %w", err)
	}
	defer func() { _ = payFile.Close() }()
	payW := csv.NewWriter(payFile)
	defer payW.Flush()

	coversFile, err := os.Create(filepath.Join("out", "cover_details.csv"))
	if err != nil {
		return fmt.Errorf("failed to create cover_details.csv: %w", err)
	}
	defer func() { _ = coversFile.Close() }()
	coversW := csv.NewWriter(coversFile)
	defer coversW.Flush()

	if err := writer.Write([]string{
		"Serial Number",
		"Title",
		"Organisation",
		"Closing Date",
		"Details URL",
		"Tender ID",

		// Basic Details section fields
		"Organisation Chain",
		"Tender Reference Number",
		"Withdrawal Allowed",
		"Tender Type",
		"Form Of Contract",
		"Tender Category",
		"Number Of Covers",
		"General Technical Evaluation Allowed",
		"ItemWise Technical Evaluation Allowed",
		"Payment Mode",
		"Is Multi Currency Allowed For BOQ",
		"Is Multi Currency Allowed For Fee",
		"Allow Two Stage Bidding",
	}); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// headers for structured CSVs
	if err := basicW.Write([]string{
		"Serial Number",
		"Title",
		"Organisation",
		"Closing Date",
		"Details URL",
		"Tender ID",
		"Organisation Chain",
		"Tender Reference Number",
		"Withdrawal Allowed",
		"Tender Type",
		"Form Of Contract",
		"Tender Category",
		"Number Of Covers",
		"General Technical Evaluation Allowed",
		"ItemWise Technical Evaluation Allowed",
		"Payment Mode",
		"Is Multi Currency Allowed For BOQ",
		"Is Multi Currency Allowed For Fee",
		"Allow Two Stage Bidding",
	}); err != nil {
		return fmt.Errorf("failed to write basic_details header: %w", err)
	}

	if err := payW.Write([]string{
		"Serial Number",
		"Tender ID",
		"Instrument Mode",
		"S.No",
		"Instrument Type",
	}); err != nil {
		return fmt.Errorf("failed to write payment_instruments header: %w", err)
	}

	if err := coversW.Write([]string{
		"Serial Number",
		"Tender ID",
		"Cover No",
		"Cover",
		"Document Type",
		"Description",
	}); err != nil {
		return fmt.Errorf("failed to write cover_details header: %w", err)
	}

	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) < 5 {
			continue
		}
		serial := strings.TrimSpace(row[0])
		title := strings.TrimSpace(row[1])
		organisation := strings.TrimSpace(row[2])
		closingDate := strings.TrimSpace(row[3])
		link := strings.TrimSpace(row[4])

		c := ts.collector.Clone()
		cookies := ts.collector.Cookies(ts.activeTendersURL)
		if len(cookies) > 0 {
			c.SetCookies(ts.activeTendersURL, cookies)
		}

		detailsURL := ""
		tenderID := ""

		// Basic Details extracted via DOM
		orgChain := ""
		tenderRef := ""
		withdrawalAllowed := ""
		tenderType := ""
		formOfContract := ""
		tenderCategory := ""
		numberOfCovers := ""
		gteAllowed := ""
		itemwiseAllowed := ""
		paymentMode := ""
		multiCurrencyBOQ := ""
		multiCurrencyFee := ""
		twoStageBidding := ""

		// Payment instruments and cover details containers
		paymentOffline := make([]utils.PaymentInstrument, 0, 4)
		covers := make([]utils.CoverInformation, 0, 8)

		// Follow both "View Tender Details" and "View More Details" anchors
		c.OnHTML("a", func(e *colly.HTMLElement) {
			text := strings.TrimSpace(e.Text)
			titleAttr := strings.TrimSpace(e.Attr("title"))
			href := e.Attr("href")
			lowerText := strings.ToLower(text)
			lowerTitle := strings.ToLower(titleAttr)
			if href != "" && (strings.Contains(lowerText, "view tender details") || strings.Contains(lowerTitle, "view more details") || strings.Contains(href, "FrontEndTenderDetails")) {
				_ = e.Request.Visit(e.Request.AbsoluteURL(href))
			}
		})

		// Parse common key/value rows on the details page
		c.OnHTML("table", func(e *colly.HTMLElement) {
			// detect a details page by presence of the Tender Details header
			if detailsURL == "" && strings.Contains(strings.ToLower(e.DOM.Text()), "tender details") {
				detailsURL = e.Request.URL.String()
			}

			// Look for the Basic Details section by checking for section_head with "Basic Details"
			sectionHead := e.DOM.Find("td.section_head").First()
			if sectionHead.Length() > 0 && strings.Contains(strings.ToLower(sectionHead.Text()), "basic details") {
				// log.Printf("Found Basic Details table")

				// Iterate through all rows with class td_caption
				e.DOM.Find("tr.td_caption").Each(func(_ int, s *goquery.Selection) {
					tds := s.Find("td")
					if tds.Length() == 0 {
						return
					}

					// Handle different row structures
					if tds.Length() == 2 {
						// Simple 2-column structure: Label | Value
						label := strings.TrimSpace(strings.TrimSuffix(tds.Eq(0).Text(), ":"))
						value := strings.TrimSpace(tds.Eq(1).Text())
						ts.assignValue(label, value, &orgChain, &tenderRef, &tenderID, &withdrawalAllowed, &tenderType, &formOfContract, &tenderCategory, &numberOfCovers, &gteAllowed, &itemwiseAllowed, &paymentMode, &multiCurrencyBOQ, &multiCurrencyFee, &twoStageBidding)
					} else if tds.Length() == 4 {
						// 4-column structure: Label1 | Value1 | Label2 | Value2
						// First pair
						label1 := strings.TrimSpace(strings.TrimSuffix(tds.Eq(0).Text(), ":"))
						value1 := strings.TrimSpace(tds.Eq(1).Text())
						ts.assignValue(label1, value1, &orgChain, &tenderRef, &tenderID, &withdrawalAllowed, &tenderType, &formOfContract, &tenderCategory, &numberOfCovers, &gteAllowed, &itemwiseAllowed, &paymentMode, &multiCurrencyBOQ, &multiCurrencyFee, &twoStageBidding)

						// Second pair
						label2 := strings.TrimSpace(strings.TrimSuffix(tds.Eq(2).Text(), ":"))
						value2 := strings.TrimSpace(tds.Eq(3).Text())
						ts.assignValue(label2, value2, &orgChain, &tenderRef, &tenderID, &withdrawalAllowed, &tenderType, &formOfContract, &tenderCategory, &numberOfCovers, &gteAllowed, &itemwiseAllowed, &paymentMode, &multiCurrencyBOQ, &multiCurrencyFee, &twoStageBidding)
					} else if tds.Length() >= 3 {
						// Handle cases where there might be colspan or other structures
						// First column is label, second is value (spanning multiple columns)
						label := strings.TrimSpace(strings.TrimSuffix(tds.Eq(0).Text(), ":"))
						value := strings.TrimSpace(tds.Eq(1).Text())
						ts.assignValue(label, value, &orgChain, &tenderRef, &tenderID, &withdrawalAllowed, &tenderType, &formOfContract, &tenderCategory, &numberOfCovers, &gteAllowed, &itemwiseAllowed, &paymentMode, &multiCurrencyBOQ, &multiCurrencyFee, &twoStageBidding)
					}
				})
			}
		})

		// Parse Payment Instruments (Offline) table by id
		c.OnHTML("table#offlineInstrumentsTableView", func(e *colly.HTMLElement) {
			// skip header row with class caption
			e.DOM.Find("tr").Each(func(_ int, s *goquery.Selection) {
				if s.HasClass("caption") {
					return
				}
				tds := s.Find("td.field_text")
				if tds.Length() >= 2 {
					serial := strings.TrimSpace(tds.Eq(0).Text())
					instr := strings.TrimSpace(tds.Eq(1).Text())
					paymentOffline = append(paymentOffline, utils.PaymentInstrument{SerialNo: serial, InstrumentType: instr})
				}
			})
		})

		// Parse Cover Details table by id
		c.OnHTML("table#packetTableView", func(e *colly.HTMLElement) {
			// rows after the header contain 4 field_text tds: CoverNo, CoverType, DocumentType, Description
			e.DOM.Find("tr").Each(func(_ int, s *goquery.Selection) {
				// skip header rows (they have class caption or contain section_head)
				if s.Find("td.section_head").Length() > 0 || s.Find("td.caption").Length() > 0 {
					return
				}
				tds := s.Find("td.field_text")
				if tds.Length() >= 4 {
					coverNo := strings.TrimSpace(tds.Eq(0).Text())
					coverType := strings.TrimSpace(tds.Eq(1).Text())
					docType := strings.TrimSpace(tds.Eq(2).Text())
					desc := strings.TrimSpace(tds.Eq(3).Text())
					covers = append(covers, utils.CoverInformation{CoverNo: coverNo, CoverType: coverType, DocumentType: docType, Description: desc})
				}
			})
		})

		// Heuristic extraction using regex as a fallback
		c.OnResponse(func(r *colly.Response) {
			body := string(r.Body)

			if detailsURL == "" && strings.Contains(strings.ToLower(body), "tender details") {
				detailsURL = r.Request.URL.String()
			}

			if tenderID == "" {
				re := regexp.MustCompile(`(?i)(Tender\s*(ID|Ref\.?\s*No\.?))\s*[:\-]?\s*([A-Za-z0-9\-/_.]+)`) // group 3
				if m := re.FindStringSubmatch(body); len(m) >= 4 {
					tenderID = strings.TrimSpace(m[3])
				}
			}
		})

		if err := c.Visit(link); err != nil {
			log.Printf("[%s] visit failed: %v", serial, err)
		}

		if err := writer.Write([]string{
			serial,
			title,
			organisation,
			closingDate,
			detailsURL,
			tenderID,
			// Basic Details
			orgChain,
			tenderRef,
			withdrawalAllowed,
			tenderType,
			formOfContract,
			tenderCategory,
			numberOfCovers,
			gteAllowed,
			itemwiseAllowed,
			paymentMode,
			multiCurrencyBOQ,
			multiCurrencyFee,
			twoStageBidding,
		}); err != nil {
			log.Printf("[%s] failed to write row: %v", serial, err)
		}
		writer.Flush()

		// structured: basic details
		if err := basicW.Write([]string{
			serial,
			title,
			organisation,
			closingDate,
			detailsURL,
			tenderID,
			orgChain,
			tenderRef,
			withdrawalAllowed,
			tenderType,
			formOfContract,
			tenderCategory,
			numberOfCovers,
			gteAllowed,
			itemwiseAllowed,
			paymentMode,
			multiCurrencyBOQ,
			multiCurrencyFee,
			twoStageBidding,
		}); err != nil {
			log.Printf("[%s] failed to write basic_details row: %v", serial, err)
		}
		basicW.Flush()

		// structured: payment instruments (offline only for now)
		for _, pi := range paymentOffline {
			if err := payW.Write([]string{serial, tenderID, "Offline", pi.SerialNo, pi.InstrumentType}); err != nil {
				log.Printf("[%s] failed to write payment_instruments row: %v", serial, err)
			}
		}
		payW.Flush()

		// structured: cover details
		for _, cv := range covers {
			if err := coversW.Write([]string{serial, tenderID, cv.CoverNo, cv.CoverType, cv.DocumentType, cv.Description}); err != nil {
				log.Printf("[%s] failed to write cover_details row: %v", serial, err)
			}
		}
		coversW.Flush()

		// also persist a structured JSONL record as {basicDetails, payment, coverDetails}
		jsonRecord := struct {
			BasicDetails struct {
				OrganisationChain                  string `json:"organisationChain"`
				TenderReferenceNumber              string `json:"tenderReferenceNumber"`
				TenderID                           string `json:"tenderId"`
				WithdrawalAllowed                  bool   `json:"withdrawalAllowed"`
				TenderType                         string `json:"tenderType"`
				FormOfContract                     string `json:"formOfContract"`
				TenderCategory                     string `json:"tenderCategory"`
				NumberOfCovers                     int    `json:"numberOfCovers"`
				GeneralTechnicalEvaluationAllowed  bool   `json:"generalTechnicalEvaluationAllowed"`
				ItemWiseTechnicalEvaluationAllowed bool   `json:"itemWiseTechnicalEvaluationAllowed"`
				PaymentMode                        string `json:"paymentMode"`
				IsMultiCurrencyAllowedForBOQ       bool   `json:"isMultiCurrencyAllowedForBOQ"`
				IsMultiCurrencyAllowedForFee       bool   `json:"isMultiCurrencyAllowedForFee"`
				AllowTwoStageBidding               bool   `json:"allowTwoStageBidding"`
			} `json:"basicDetails"`
			Payment struct {
				Offline []utils.PaymentInstrument `json:"offline"`
				Online  []utils.PaymentInstrument `json:"online"`
			} `json:"payment"`
			CoverDetails []utils.CoverInformation `json:"coverDetails"`
		}{}

		jsonRecord.BasicDetails.OrganisationChain = orgChain
		jsonRecord.BasicDetails.TenderReferenceNumber = tenderRef
		jsonRecord.BasicDetails.TenderID = tenderID
		jsonRecord.BasicDetails.WithdrawalAllowed = strings.EqualFold(strings.TrimSpace(withdrawalAllowed), "yes")
		jsonRecord.BasicDetails.TenderType = tenderType
		jsonRecord.BasicDetails.FormOfContract = formOfContract
		jsonRecord.BasicDetails.TenderCategory = tenderCategory
		if n := strings.TrimSpace(numberOfCovers); n != "" {
			if n == "1" {
				jsonRecord.BasicDetails.NumberOfCovers = 1
			} else if n == "2" {
				jsonRecord.BasicDetails.NumberOfCovers = 2
			}
		}
		jsonRecord.BasicDetails.GeneralTechnicalEvaluationAllowed = strings.EqualFold(strings.TrimSpace(gteAllowed), "yes")
		jsonRecord.BasicDetails.ItemWiseTechnicalEvaluationAllowed = strings.EqualFold(strings.TrimSpace(itemwiseAllowed), "yes")
		jsonRecord.BasicDetails.PaymentMode = paymentMode
		jsonRecord.BasicDetails.IsMultiCurrencyAllowedForBOQ = strings.EqualFold(strings.TrimSpace(multiCurrencyBOQ), "yes")
		jsonRecord.BasicDetails.IsMultiCurrencyAllowedForFee = strings.EqualFold(strings.TrimSpace(multiCurrencyFee), "yes")
		jsonRecord.BasicDetails.AllowTwoStageBidding = strings.EqualFold(strings.TrimSpace(twoStageBidding), "yes")

		jsonRecord.Payment.Offline = paymentOffline
		// Online can be filled when we parse its table in future
		jsonRecord.Payment.Online = nil
		jsonRecord.CoverDetails = covers

		if err := utils.AppendJSONL("out/tenders.jsonl", jsonRecord); err != nil {
			log.Printf("[%s] failed to append JSONL: %v", serial, err)
		}
	}

	log.Println("Tender data extraction completed.")
	return nil
}

// assignValue maps a Basic Details label to the corresponding target field pointer.
func (ts *TenderDataScraper) assignValue(label, value string, orgChain, tenderRef, tenderID, withdrawalAllowed, tenderType, formOfContract, tenderCategory, numberOfCovers, gteAllowed, itemwiseAllowed, paymentMode, multiCurrencyBOQ, multiCurrencyFee, twoStageBidding *string) {
	labelLower := strings.ToLower(strings.TrimSpace(label))
	// Remove common formatting characters
	labelLower = strings.ReplaceAll(labelLower, ".", "")

	switch labelLower {
	case "organisation chain":
		if *orgChain == "" {
			*orgChain = value
		}
	case "tender reference number":
		if *tenderRef == "" {
			*tenderRef = value
		}
	case "tender id":
		if *tenderID == "" {
			*tenderID = value
		}
	case "withdrawal allowed":
		if *withdrawalAllowed == "" {
			*withdrawalAllowed = value
		}
	case "tender type":
		if *tenderType == "" {
			*tenderType = value
		}
	case "form of contract":
		if *formOfContract == "" {
			*formOfContract = value
		}
	case "tender category":
		if *tenderCategory == "" {
			*tenderCategory = value
		}
	case "no of covers", "number of covers":
		if *numberOfCovers == "" {
			*numberOfCovers = value
		}
	case "general technical evaluation allowed":
		if *gteAllowed == "" {
			*gteAllowed = value
		}
	case "itemwise technical evaluation allowed":
		if *itemwiseAllowed == "" {
			*itemwiseAllowed = value
		}
	case "payment mode":
		if *paymentMode == "" {
			*paymentMode = value
		}
	case "is multi currency allowed for boq":
		if *multiCurrencyBOQ == "" {
			*multiCurrencyBOQ = value
		}
	case "is multi currency allowed for fee":
		if *multiCurrencyFee == "" {
			*multiCurrencyFee = value
		}
	case "allow two stage bidding":
		if *twoStageBidding == "" {
			*twoStageBidding = value
		}
	}
}
