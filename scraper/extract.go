package scraper

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

// csvSinks groups structured CSV writers
type csvSinks struct {
	basicW    *csv.Writer
	payW      *csv.Writer
	coversW   *csv.Writer
	feeW      *csv.Writer
	emdW      *csv.Writer
	workW     *csv.Writer
	critW     *csv.Writer
	docsNitW  *csv.Writer
	docsWorkW *csv.Writer
}

// setupStructuredCSVs creates files under out/ and writes headers.
// Returns sinks and a cleanup closer that closes files and flushes writers.
func (ts *TenderDataScraper) setupStructuredCSVs() (csvSinks, func(), error) {
	if err := os.MkdirAll("out", 0755); err != nil {
		return csvSinks{}, nil, fmt.Errorf("failed to create out directory: %w", err)
	}

	basicFile, err := os.Create(filepath.Join("out", "basic_details.csv"))
	if err != nil {
		return csvSinks{}, nil, fmt.Errorf("failed to create basic_details.csv: %w", err)
	}
	payFile, err := os.Create(filepath.Join("out", "payment_instruments.csv"))
	if err != nil {
		_ = basicFile.Close()
		return csvSinks{}, nil, fmt.Errorf("failed to create payment_instruments.csv: %w", err)
	}
	coversFile, err := os.Create(filepath.Join("out", "cover_details.csv"))
	if err != nil {
		_ = basicFile.Close()
		_ = payFile.Close()
		return csvSinks{}, nil, fmt.Errorf("failed to create cover_details.csv: %w", err)
	}
	feeFile, err := os.Create(filepath.Join("out", "tender_fee_details.csv"))
	if err != nil {
		_ = basicFile.Close()
		_ = payFile.Close()
		_ = coversFile.Close()
		return csvSinks{}, nil, fmt.Errorf("failed to create tender_fee_details.csv: %w", err)
	}
	emdFile, err := os.Create(filepath.Join("out", "emd_fee_details.csv"))
	if err != nil {
		_ = basicFile.Close()
		_ = payFile.Close()
		_ = coversFile.Close()
		_ = feeFile.Close()
		return csvSinks{}, nil, fmt.Errorf("failed to create emd_fee_details.csv: %w", err)
	}
	workFile, err := os.Create(filepath.Join("out", "work_item_details.csv"))
	if err != nil {
		_ = basicFile.Close()
		_ = payFile.Close()
		_ = coversFile.Close()
		_ = feeFile.Close()
		_ = emdFile.Close()
		return csvSinks{}, nil, fmt.Errorf("failed to create work_item_details.csv: %w", err)
	}
	critFile, err := os.Create(filepath.Join("out", "critical_dates.csv"))
	if err != nil {
		_ = basicFile.Close()
		_ = payFile.Close()
		_ = coversFile.Close()
		_ = feeFile.Close()
		_ = emdFile.Close()
		_ = workFile.Close()
		return csvSinks{}, nil, fmt.Errorf("failed to create critical_dates.csv: %w", err)
	}
	docsNitFile, err := os.Create(filepath.Join("out", "tender_documents_nit.csv"))
	if err != nil {
		_ = basicFile.Close()
		_ = payFile.Close()
		_ = coversFile.Close()
		_ = feeFile.Close()
		_ = emdFile.Close()
		_ = workFile.Close()
		_ = critFile.Close()
		return csvSinks{}, nil, fmt.Errorf("failed to create tender_documents_nit.csv: %w", err)
	}
	docsWorkFile, err := os.Create(filepath.Join("out", "tender_documents_workitem.csv"))
	if err != nil {
		_ = basicFile.Close()
		_ = payFile.Close()
		_ = coversFile.Close()
		_ = feeFile.Close()
		_ = emdFile.Close()
		_ = workFile.Close()
		_ = critFile.Close()
		_ = docsNitFile.Close()
		return csvSinks{}, nil, fmt.Errorf("failed to create tender_documents_workitem.csv: %w", err)
	}
	

	basicW := csv.NewWriter(basicFile)
	payW := csv.NewWriter(payFile)
	coversW := csv.NewWriter(coversFile)
	feeW := csv.NewWriter(feeFile)
	emdW := csv.NewWriter(emdFile)
	workW := csv.NewWriter(workFile)
	critW := csv.NewWriter(critFile)
	docsNitW := csv.NewWriter(docsNitFile)
	docsWorkW := csv.NewWriter(docsWorkFile)

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
		_ = basicFile.Close()
		_ = payFile.Close()
		_ = coversFile.Close()
		_ = feeFile.Close()
		_ = emdFile.Close()
		_ = workFile.Close()
		_ = critFile.Close()
		_ = docsNitFile.Close()
		_ = docsWorkFile.Close()
		return csvSinks{}, nil, fmt.Errorf("failed to write basic_details header: %w", err)
	}

	if err := payW.Write([]string{
		"Serial Number",
		"Tender ID",
		"Instrument Mode",
		"S.No",
		"Instrument Type",
	}); err != nil {
		_ = basicFile.Close()
		_ = payFile.Close()
		_ = coversFile.Close()
		_ = feeFile.Close()
		_ = emdFile.Close()
		_ = workFile.Close()
		_ = critFile.Close()
		_ = docsNitFile.Close()
		_ = docsWorkFile.Close()
		return csvSinks{}, nil, fmt.Errorf("failed to write payment_instruments header: %w", err)
	}

	if err := coversW.Write([]string{
		"Serial Number",
		"Tender ID",
		"Cover No",
		"Cover",
		"Document Type",
		"Description",
	}); err != nil {
		_ = basicFile.Close()
		_ = payFile.Close()
		_ = coversFile.Close()
		_ = feeFile.Close()
		_ = emdFile.Close()
		_ = workFile.Close()
		_ = critFile.Close()
		_ = docsNitFile.Close()
		_ = docsWorkFile.Close()
		return csvSinks{}, nil, fmt.Errorf("failed to write cover_details header: %w", err)
	}

	if err := feeW.Write([]string{
		"Serial Number",
		"Tender ID",
		"Total Fee",
		"Tender Fee",
		"Fee Payable To",
		"Fee Payable At",
		"Tender Fee Exemption Allowed",
	}); err != nil {
		_ = basicFile.Close()
		_ = payFile.Close()
		_ = coversFile.Close()
		_ = feeFile.Close()
		_ = emdFile.Close()
		_ = workFile.Close()
		_ = critFile.Close()
		_ = docsNitFile.Close()
		_ = docsWorkFile.Close()
		return csvSinks{}, nil, fmt.Errorf("failed to write tender_fee_details header: %w", err)
	}

	if err := emdW.Write([]string{
		"Serial Number",
		"Tender ID",
		"EMD Amount",
		"EMD Exemption Allowed",
		"EMD Fee Type",
		"EMD Percentage",
		"EMD Payable To",
		"EMD Payable At",
	}); err != nil {
		_ = basicFile.Close()
		_ = payFile.Close()
		_ = coversFile.Close()
		_ = feeFile.Close()
		_ = emdFile.Close()
		_ = workFile.Close()
		_ = critFile.Close()
		_ = docsNitFile.Close()
		_ = docsWorkFile.Close()
		return csvSinks{}, nil, fmt.Errorf("failed to write emd_fee_details header: %w", err)
	}

	if err := workW.Write([]string{
		"Serial Number",
		"Tender ID",
		"Title",
		"Description",
		"PreQualification",
		"IndependentExternalMonitorRemarks",
		"TenderValue",
		"ProductCategory",
		"SubCategory",
		"ContractType",
		"BidValidityDays",
		"PeriodOfWorkDays",
		"Location",
		"Pincode",
		"PreBidMeetingPlace",
		"PreBidMeetingAddress",
		"PreBidMeetingDate",
		"BidOpeningPlace",
		"ShouldAllowNDATender",
		"AllowPreferentialBidder",
	}); err != nil {
		_ = basicFile.Close()
		_ = payFile.Close()
		_ = coversFile.Close()
		_ = feeFile.Close()
		_ = emdFile.Close()
		_ = workFile.Close()
		_ = critFile.Close()
		_ = docsNitFile.Close()
		_ = docsWorkFile.Close()
		return csvSinks{}, nil, fmt.Errorf("failed to write work_item_details header: %w", err)
	}

	if err := critW.Write([]string{
		"Serial Number",
		"Tender ID",
		"PublishedDate",
		"BidOpeningDate",
		"DocumentDownloadStartDate",
		"DocumentDownloadEndDate",
		"ClarificationStartDate",
		"ClarificationEndDate",
		"BidSubmissionStartDate",
		"BidSubmissionEndDate",
	}); err != nil {
		_ = basicFile.Close()
		_ = payFile.Close()
		_ = coversFile.Close()
		_ = feeFile.Close()
		_ = emdFile.Close()
		_ = workFile.Close()
		_ = critFile.Close()
		_ = docsNitFile.Close()
		_ = docsWorkFile.Close()
		return csvSinks{}, nil, fmt.Errorf("failed to write critical_dates header: %w", err)
	}

	if err := docsNitW.Write([]string{
		"Serial Number",
		"Tender ID",
		"S.No",
		"Document Name",
		"Description",
		"Document Size (KB)",
	}); err != nil {
		return csvSinks{}, nil, fmt.Errorf("failed to write tender_documents_nit header: %w", err)
	}

	if err := docsWorkW.Write([]string{
		"Serial Number",
		"Tender ID",
		"S.No",
		"Document Type",
		"Document Name",
		"Description",
		"Document Size (KB)",
	}); err != nil {
		return csvSinks{}, nil, fmt.Errorf("failed to write tender_documents_workitem header: %w", err)
	}

	closer := func() {
		basicW.Flush()
		payW.Flush()
		coversW.Flush()
		feeW.Flush()
		emdW.Flush()
		workW.Flush()
		critW.Flush()
		docsNitW.Flush()
		docsWorkW.Flush()
		_ = basicFile.Close()
		_ = payFile.Close()
		_ = coversFile.Close()
		_ = feeFile.Close()
		_ = emdFile.Close()
		_ = workFile.Close()
		_ = critFile.Close()
		// nit/work files closed by their writers' underlying files when program exits; best-effort flush above
	}

	return csvSinks{basicW: basicW, payW: payW, coversW: coversW, feeW: feeW, emdW: emdW, workW: workW, critW: critW, docsNitW: docsNitW, docsWorkW: docsWorkW}, closer, nil
}

// write helpers for structured CSVs
func (s csvSinks) writeBasicDetails(serial, title, organisation, closingDate, detailsURL, tenderID, orgChain, tenderRef, withdrawalAllowed, tenderType, formOfContract, tenderCategory, numberOfCovers, gteAllowed, itemwiseAllowed, paymentMode, multiCurrencyBOQ, multiCurrencyFee, twoStageBidding string) {
	_ = s.basicW.Write([]string{serial, title, organisation, closingDate, detailsURL, tenderID, orgChain, tenderRef, withdrawalAllowed, tenderType, formOfContract, tenderCategory, numberOfCovers, gteAllowed, itemwiseAllowed, paymentMode, multiCurrencyBOQ, multiCurrencyFee, twoStageBidding})
	s.basicW.Flush()
}

func (s csvSinks) writePaymentInstruments(serial, tenderID string, mode string, items []utils.PaymentInstrument) {
	for _, pi := range items {
		_ = s.payW.Write([]string{serial, tenderID, mode, pi.SerialNo, pi.InstrumentType})
	}
	s.payW.Flush()
}

func (s csvSinks) writeCoverDetails(serial, tenderID string, covers []utils.CoverInformation) {
	for _, cv := range covers {
		_ = s.coversW.Write([]string{serial, tenderID, cv.CoverNo, cv.CoverType, cv.DocumentType, cv.Description})
	}
	s.coversW.Flush()
}

func (s csvSinks) writeTenderFee(serial, tenderID, totalFee, tenderFee, feePayableTo, feePayableAt, tenderFeeExemptionAllowed string) {
	_ = s.feeW.Write([]string{serial, tenderID, totalFee, tenderFee, feePayableTo, feePayableAt, tenderFeeExemptionAllowed})
	s.feeW.Flush()
}

func (s csvSinks) writeEmd(serial, tenderID, emdAmount, emdExemptionAllowed, emdFeeType, emdPercentage, emdPayableTo, emdPayableAt string) {
	_ = s.emdW.Write([]string{serial, tenderID, emdAmount, emdExemptionAllowed, emdFeeType, emdPercentage, emdPayableTo, emdPayableAt})
	s.emdW.Flush()
}

func (s csvSinks) writeWorkItem(serial, tenderID string, wi workItemParsed) {
	_ = s.workW.Write([]string{serial, tenderID, wi.Title, wi.Description, wi.PreQualification, wi.IEMRemarks, wi.TenderValue, wi.ProductCategory, wi.SubCategory, wi.ContractType, wi.BidValidityDays, wi.PeriodOfWorkDays, wi.Location, wi.Pincode, wi.PreBidMeetingPlace, wi.PreBidMeetingAddress, wi.PreBidMeetingDate, wi.BidOpeningPlace, wi.ShouldAllowNDA, wi.AllowPreferentialBidder})
	s.workW.Flush()
}

// local parsed holder for Critical Dates
type criticalDatesParsed struct {
	PublishedDate             string
	BidOpeningDate            string
	DocumentDownloadStartDate string
	DocumentDownloadEndDate   string
	ClarificationStartDate    *string
	ClarificationEndDate      *string
	BidSubmissionStartDate    string
	BidSubmissionEndDate      string
}

func (s csvSinks) writeCriticalDates(serial, tenderID string, cd criticalDatesParsed) {
	val := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	_ = s.critW.Write([]string{serial, tenderID, cd.PublishedDate, cd.BidOpeningDate, cd.DocumentDownloadStartDate, cd.DocumentDownloadEndDate, val(cd.ClarificationStartDate), val(cd.ClarificationEndDate), cd.BidSubmissionStartDate, cd.BidSubmissionEndDate})
	s.critW.Flush()
}

// ephemeral parsed holder for Work/Item(s)
type workItemParsed struct {
	Title                   string
	Description             string
	PreQualification        string
	IEMRemarks              string
	TenderValue             string
	ProductCategory         string
	SubCategory             string
	ContractType            string
	BidValidityDays         string
	PeriodOfWorkDays        string
	Location                string
	Pincode                 string
	PreBidMeetingPlace      string
	PreBidMeetingAddress    string
	PreBidMeetingDate       string
	BidOpeningPlace         string
	ShouldAllowNDA          string
	AllowPreferentialBidder string
}

func (s csvSinks) writeNitDocs(serial, tenderID string, items []nitDocParsed) {
	for _, d := range items {
		_ = s.docsNitW.Write([]string{serial, tenderID, d.SerialNo, d.DocumentName, d.Description, d.DocumentSizeKB})
	}
	s.docsNitW.Flush()
}

func (s csvSinks) writeWorkItemDocs(serial, tenderID string, items []workDocParsed) {
	for _, d := range items {
		_ = s.docsWorkW.Write([]string{serial, tenderID, d.SerialNo, d.DocumentType, d.DocumentName, d.Description, d.DocumentSizeKB})
	}
	s.docsWorkW.Flush()
}

// local parsed holders for documents
type nitDocParsed struct {
	SerialNo       string
	DocumentName   string
	Description    string
	DocumentSizeKB string
}

type workDocParsed struct {
	SerialNo       string
	DocumentType   string
	DocumentName   string
	Description    string
	DocumentSizeKB string
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

	// structured CSV setup
	sinks, closeSinks, err := ts.setupStructuredCSVs()
	if err != nil {
		return err
	}
	defer closeSinks()

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

		// Tender Documents containers
		nitDocs := make([]nitDocParsed, 0, 4)
		workDocs := make([]workDocParsed, 0, 4)

		// Tender Fee Details
		totalFee := ""
		tenderFee := ""
		feePayableTo := ""
		feePayableAt := ""
		tenderFeeExemptionAllowed := ""

		// EMD Fee Details
		emdAmount := ""
		emdExemptionAllowed := ""
		emdFeeType := ""
		emdPercentage := ""
		emdPayableTo := ""
		emdPayableAt := ""

		// Work/Item(s) details holder
		work := workItemParsed{}

		// Critical Dates holder
		critical := criticalDatesParsed{}

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

		// Parse Tender Documents - NIT table (search by nested context header and inner table rows)
		c.OnHTML("td.section_head", func(e *colly.HTMLElement) {
			head := strings.ToLower(strings.TrimSpace(e.DOM.Text()))
			if strings.Contains(head, "tender documents") {
				parent := e.DOM.Closest("table")
				// NIT table may have id="table" inside field
				parent.Find("table#table").Find("tr").Each(func(_ int, s *goquery.Selection) {
					// skip header with caption
					if s.Find("td.caption").Length() > 0 {
						return
					}
					tds := s.Find("td")
					if tds.Length() >= 4 {
						sn := strings.TrimSpace(tds.Eq(0).Text())
						name := strings.TrimSpace(tds.Eq(1).Text())
						desc := strings.TrimSpace(tds.Eq(2).Text())
						size := strings.TrimSpace(tds.Eq(3).Text())
						if sn != "" && name != "" {
							nitDocs = append(nitDocs, nitDocParsed{SerialNo: sn, DocumentName: name, Description: desc, DocumentSizeKB: size})
						}
					}
				})
				// Work Item Documents table id="workItemDocumenttable"
				parent.Find("table#workItemDocumenttable").Find("tr").Each(func(_ int, s *goquery.Selection) {
					if s.Find("td.caption").Length() > 0 {
						return
					}
					tds := s.Find("td")
					if tds.Length() >= 5 {
						sn := strings.TrimSpace(tds.Eq(0).Text())
						typeTxt := strings.TrimSpace(tds.Eq(1).Text())
						name := strings.TrimSpace(tds.Eq(2).Text())
						desc := strings.TrimSpace(tds.Eq(3).Text())
						size := strings.TrimSpace(tds.Eq(4).Text())
						if sn != "" && (name != "" || typeTxt != "") {
							workDocs = append(workDocs, workDocParsed{SerialNo: sn, DocumentType: typeTxt, DocumentName: name, Description: desc, DocumentSizeKB: size})
						}
					}
				})
			}
		})

		// Parse Tender Fee Details block
		c.OnHTML("td.section_head", func(e *colly.HTMLElement) {
			head := strings.ToLower(strings.TrimSpace(e.DOM.Text()))
			if strings.Contains(head, "tender fee details") {
				// Extract total fee from header text if present
				if m := regexp.MustCompile(`(?i)total\s*fee[^0-9]*([0-9,]+\.?[0-9]*)`).FindStringSubmatch(e.DOM.Text()); len(m) >= 2 {
					totalFee = strings.TrimSpace(m[1])
				}
				parentTable := e.DOM.Closest("table")
				parentTable.Find("tr").Each(func(_ int, s *goquery.Selection) {
					capCell := strings.ToLower(strings.TrimSpace(s.Find("td.caption").First().Text()))
					if capCell == "tender fee in ₹" || strings.Contains(capCell, "tender fee in") {
						val := strings.TrimSpace(s.Find("td.view_list_field, td.field_text").First().Text())
						tenderFee = val
					}
					tds := s.Find("td.field_text")
					if tds.Length() >= 2 && strings.Contains(strings.ToLower(strings.TrimSpace(s.Text())), "fee payable") {
						feePayableTo = strings.TrimSpace(tds.Eq(0).Text())
						feePayableAt = strings.TrimSpace(tds.Eq(1).Text())
					}
					if strings.Contains(strings.ToLower(strings.TrimSpace(s.Text())), "tender fee exemption allowed") {
						val := strings.TrimSpace(s.Find("td.field_text").First().Text())
						tenderFeeExemptionAllowed = val
					}
				})
			}
		})

		// Parse EMD Fee Details block
		c.OnHTML("td.section_head", func(e *colly.HTMLElement) {
			head := strings.ToLower(strings.TrimSpace(e.DOM.Text()))
			if strings.Contains(head, "emd fee details") {
				parentTable := e.DOM.Closest("table")
				parentTable.Find("tr").Each(func(_ int, s *goquery.Selection) {
					cells := s.Find("td")
					if cells.Length() >= 2 {
						cap := strings.ToLower(strings.TrimSpace(cells.Eq(0).Text()))
						val := strings.TrimSpace(cells.Eq(1).Text())
						if strings.Contains(cap, "emd amount") {
							emdAmount = val
						}
						if strings.Contains(cap, "fee type") {
							emdFeeType = val
						}
						if strings.Contains(cap, "emd payable to") {
							emdPayableTo = val
						}
					}
					if cells.Length() >= 4 {
						cap2 := strings.ToLower(strings.TrimSpace(cells.Eq(2).Text()))
						val2 := strings.TrimSpace(cells.Eq(3).Text())
						if strings.Contains(cap2, "exemption") {
							emdExemptionAllowed = val2
						}
						if strings.Contains(cap2, "percentage") {
							emdPercentage = val2
						}
						if strings.Contains(cap2, "emd payable at") {
							emdPayableAt = val2
						}
					}
				})
			}
		})

		// Parse Critical Dates block
		c.OnHTML("td.section_head", func(e *colly.HTMLElement) {
			head := strings.ToLower(strings.TrimSpace(e.DOM.Text()))
			if strings.Contains(head, "critical dates") {
				parentTable := e.DOM.Closest("table")
				parentTable.Find("tr").Each(func(_ int, s *goquery.Selection) {
					cap := strings.ToLower(strings.TrimSpace(s.Find("td.caption").First().Text()))
					valField := s.Find("td.field_text").First()
					val := strings.TrimSpace(valField.Text())
					switch cap {
					case "published date":
						critical.PublishedDate = val
					case "bid opening date":
						critical.BidOpeningDate = val
					case "document download start date":
						critical.DocumentDownloadStartDate = val
					case "document download end date":
						critical.DocumentDownloadEndDate = val
					case "clarification start date":
						clarificationStartDate := strings.TrimSpace(val)
						if clarificationStartDate != "" && !strings.EqualFold(clarificationStartDate, "na") {
							critical.ClarificationStartDate = &clarificationStartDate
						}
					case "clarification end date":
						clarificationEndDate := strings.TrimSpace(val)
						if clarificationEndDate != "" && !strings.EqualFold(clarificationEndDate, "na") {
							critical.ClarificationEndDate = &clarificationEndDate
						}
					case "bid submission start date":
						critical.BidSubmissionStartDate = val
					case "bid submission end date":
						critical.BidSubmissionEndDate = val
					}
				})
			}
		})

		// Parse Work/Item(s) table
		c.OnHTML("td.section_head", func(e *colly.HTMLElement) {
			head := strings.ToLower(strings.TrimSpace(e.DOM.Text()))
			if strings.Contains(head, "work /item") {
				parentTable := e.DOM.Closest("table")
				parentTable.Find("tr").Each(func(_ int, s *goquery.Selection) {
					cap := strings.ToLower(strings.TrimSpace(s.Find("td.caption").First().Text()))
					valField := s.Find("td.field_text").First()
					val := strings.TrimSpace(valField.Text())
					switch cap {
					case "title":
						work.Title = val
					case "work description":
						work.Description = val
					case "pre qualification details":
						work.PreQualification = val
					case "independent external monitor/remarks":
						work.IEMRemarks = val
					case "tender value in ₹":
						work.TenderValue = strings.TrimSpace(val)
					case "product category":
						work.ProductCategory = val
					case "sub category":
						work.SubCategory = val
					case "contract type":
						work.ContractType = val
					case "bid validity(days)":
						work.BidValidityDays = val
					case "period of work(days)":
						work.PeriodOfWorkDays = val
					case "location":
						work.Location = val
					case "pincode":
						work.Pincode = val
					case "pre bid meeting place":
						work.PreBidMeetingPlace = val
					case "pre bid meeting address":
						work.PreBidMeetingAddress = val
					case "pre bid meeting date":
						work.PreBidMeetingDate = val
					case "bid opening place":
						work.BidOpeningPlace = val
					case "should allow nda tender":
						work.ShouldAllowNDA = strings.TrimSpace(val)
					case "allow preferential bidder":
						work.AllowPreferentialBidder = strings.TrimSpace(val)
					}
				})
			}
		})

		// Parse Critical Dates table
		c.OnHTML("td.section_head", func(e *colly.HTMLElement) {
			head := strings.ToLower(strings.TrimSpace(e.DOM.Text()))
			if strings.Contains(head, "critical dates") {
				parentTable := e.DOM.Closest("table")
				parentTable.Find("tr").Each(func(_ int, s *goquery.Selection) {
					cells := s.Find("td")
					if cells.Length() >= 2 {
						cap := strings.ToLower(strings.TrimSpace(cells.Eq(0).Text()))
						val := strings.TrimSpace(cells.Eq(1).Text())
						switch cap {
						case "publish date":
							critical.PublishedDate = val
						case "bid opening date":
							critical.BidOpeningDate = val
						case "document download / sale start date":
							critical.DocumentDownloadStartDate = val
						case "document download / sale end date":
							critical.DocumentDownloadEndDate = val
						case "clarification start date":
							if v := strings.TrimSpace(val); v != "" && !strings.EqualFold(v, "na") {
								critical.ClarificationStartDate = &v
							}
						case "clarification end date":
							if v := strings.TrimSpace(val); v != "" && !strings.EqualFold(v, "na") {
								critical.ClarificationEndDate = &v
							}
						case "bid submission start date":
							critical.BidSubmissionStartDate = val
						case "bid submission end date":
							critical.BidSubmissionEndDate = val
						}
					}
				})
			}
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

		// structured CSV writes via sinks
		sinks.writeBasicDetails(serial, title, organisation, closingDate, detailsURL, tenderID, orgChain, tenderRef, withdrawalAllowed, tenderType, formOfContract, tenderCategory, numberOfCovers, gteAllowed, itemwiseAllowed, paymentMode, multiCurrencyBOQ, multiCurrencyFee, twoStageBidding)
		sinks.writePaymentInstruments(serial, tenderID, "Offline", paymentOffline)
		sinks.writeCoverDetails(serial, tenderID, covers)
		sinks.writeTenderFee(serial, tenderID, totalFee, tenderFee, feePayableTo, feePayableAt, tenderFeeExemptionAllowed)
		sinks.writeEmd(serial, tenderID, emdAmount, emdExemptionAllowed, emdFeeType, emdPercentage, emdPayableTo, emdPayableAt)
		sinks.writeWorkItem(serial, tenderID, work)
		sinks.writeCriticalDates(serial, tenderID, critical)
		sinks.writeNitDocs(serial, tenderID, nitDocs)
		sinks.writeWorkItemDocs(serial, tenderID, workDocs)

		// JSONL record using utils.Tender
		tender := utils.Tender{}
		tender.BasicDetails.OrganisationChain = orgChain
		tender.BasicDetails.TenderReferenceNumber = tenderRef
		tender.BasicDetails.TenderID = tenderID
		tender.BasicDetails.WithdrawalAllowed = strings.EqualFold(strings.TrimSpace(withdrawalAllowed), "yes")
		tender.BasicDetails.TenderType = tenderType
		tender.BasicDetails.FormOfContract = formOfContract
		tender.BasicDetails.TenderCategory = tenderCategory
		if n := strings.TrimSpace(numberOfCovers); n != "" {
			switch n {
			case "1":
				tender.BasicDetails.NumberOfCovers = 1
			case "2":
				tender.BasicDetails.NumberOfCovers = 2
			}
		}
		tender.BasicDetails.GeneralTechnicalEvaluationAllowed = strings.EqualFold(strings.TrimSpace(gteAllowed), "yes")
		tender.BasicDetails.ItemWiseTechnicalEvaluationAllowed = strings.EqualFold(strings.TrimSpace(itemwiseAllowed), "yes")
		tender.BasicDetails.PaymentMode = paymentMode
		tender.BasicDetails.IsMultiCurrencyAllowedForBOQ = strings.EqualFold(strings.TrimSpace(multiCurrencyBOQ), "yes")
		tender.BasicDetails.IsMultiCurrencyAllowedForFee = strings.EqualFold(strings.TrimSpace(multiCurrencyFee), "yes")
		tender.BasicDetails.AllowTwoStageBidding = strings.EqualFold(strings.TrimSpace(twoStageBidding), "yes")

		tender.PaymentInstruments.Offline = paymentOffline
		// Online instruments can be added when parsed

		tender.CoversInformation = covers

		tender.TenderFeeDetails.TotalFee = parseAmountToFloat(totalFee)
		tender.TenderFeeDetails.TenderFee = parseAmountToFloat(tenderFee)
		tender.TenderFeeDetails.FeePayableTo = feePayableTo
		tender.TenderFeeDetails.FeePayableAt = feePayableAt
		tender.TenderFeeDetails.TenderFeeExemptionAllowed = strings.EqualFold(strings.TrimSpace(tenderFeeExemptionAllowed), "yes")

		tender.EmdFeeDetails.EmdAmount = parseAmountToFloat(emdAmount)
		tender.EmdFeeDetails.EmdExemptionAllowed = strings.EqualFold(strings.TrimSpace(emdExemptionAllowed), "yes")
		tender.EmdFeeDetails.EmdFeeType = emdFeeType
		if strings.TrimSpace(emdPercentage) != "" && !strings.EqualFold(strings.TrimSpace(emdPercentage), "na") {
			pct := strings.TrimSpace(emdPercentage)
			tender.EmdFeeDetails.EmdPercentage = &pct
		}
		tender.EmdFeeDetails.EmdPayableTo = emdPayableTo
		tender.EmdFeeDetails.EmdPayableAt = emdPayableAt

		// Map Work/Item(s)
		tender.WorkItemDetails.Title = work.Title
		tender.WorkItemDetails.Description = work.Description
		tender.WorkItemDetails.NDAOrPreQualification = work.PreQualification
		tender.WorkItemDetails.IndependentExternalMonitorRemarks = work.IEMRemarks
		tender.WorkItemDetails.TenderValue = parseAmountToFloat(work.TenderValue)
		tender.WorkItemDetails.ProductCategory = work.ProductCategory
		if ws := strings.TrimSpace(work.SubCategory); ws != "" && !strings.EqualFold(ws, "na") {
			tender.WorkItemDetails.SubCategory = &ws
		}
		tender.WorkItemDetails.ContractType = work.ContractType
		if d, err := strconv.Atoi(strings.TrimSpace(work.BidValidityDays)); err == nil {
			tender.WorkItemDetails.BidValidityDays = d
		}
		if pd := strings.TrimSpace(work.PeriodOfWorkDays); pd != "" && !strings.EqualFold(pd, "na") {
			if v, err := strconv.Atoi(pd); err == nil {
				tender.WorkItemDetails.PeriodOfWorkDays = &v
			}
		}
		tender.WorkItemDetails.Location = work.Location
		tender.WorkItemDetails.Pincode = work.Pincode
		if v := strings.TrimSpace(work.PreBidMeetingPlace); v != "" && !strings.EqualFold(v, "na") {
			tender.WorkItemDetails.PreBidMeetingPlace = &v
		}
		if v := strings.TrimSpace(work.PreBidMeetingAddress); v != "" && !strings.EqualFold(v, "na") {
			tender.WorkItemDetails.PreBidMeetingAddress = &v
		}
		if v := strings.TrimSpace(work.PreBidMeetingDate); v != "" && !strings.EqualFold(v, "na") {
			tender.WorkItemDetails.PreBidMeetingDate = &v
		}
		tender.WorkItemDetails.BidOpeningPlace = work.BidOpeningPlace
		tender.WorkItemDetails.ShouldAllowNDATender = strings.EqualFold(strings.TrimSpace(work.ShouldAllowNDA), "yes")
		tender.WorkItemDetails.AllowPreferentialBidder = strings.EqualFold(strings.TrimSpace(work.AllowPreferentialBidder), "yes")

		// Critical Dates JSON mapping
		tender.CriticalDates.PublishedDate = critical.PublishedDate
		tender.CriticalDates.BidOpeningDate = critical.BidOpeningDate
		tender.CriticalDates.DocumentDownloadStartDate = critical.DocumentDownloadStartDate
		tender.CriticalDates.DocumentDownloadEndDate = critical.DocumentDownloadEndDate
		tender.CriticalDates.ClarificationStartDate = critical.ClarificationStartDate
		tender.CriticalDates.ClarificationEndDate = critical.ClarificationEndDate
		tender.CriticalDates.BidSubmissionStartDate = critical.BidSubmissionStartDate
		tender.CriticalDates.BidSubmissionEndDate = critical.BidSubmissionEndDate

		// Tender Documents JSON mapping
		if len(nitDocs) > 0 || len(workDocs) > 0 {
			entry := struct {
				WorkItemDocuments []struct {
					SerialNo       string
					DocumentType   string
					DocumentName   string
					Description    string
					DocumentSizeKB float64
				}
				NITDocuments []struct {
					SerialNo       string
					DocumentName   string
					Description    string
					DocumentSizeKB float64
				}
			}{}
			for _, d := range workDocs {
				entry.WorkItemDocuments = append(entry.WorkItemDocuments, struct {
					SerialNo       string
					DocumentType   string
					DocumentName   string
					Description    string
					DocumentSizeKB float64
				}{SerialNo: d.SerialNo, DocumentType: d.DocumentType, DocumentName: d.DocumentName, Description: d.Description, DocumentSizeKB: parseAmountToFloat(d.DocumentSizeKB)})
			}
			for _, d := range nitDocs {
				entry.NITDocuments = append(entry.NITDocuments, struct {
					SerialNo       string
					DocumentName   string
					Description    string
					DocumentSizeKB float64
				}{SerialNo: d.SerialNo, DocumentName: d.DocumentName, Description: d.Description, DocumentSizeKB: parseAmountToFloat(d.DocumentSizeKB)})
			}
			// append single combined entry to TenderDocuments slice
			tender.TenderDocuments = append(tender.TenderDocuments, struct {
				WorkItemDocuments []struct {
					SerialNo       string
					DocumentType   string
					DocumentName   string
					Description    string
					DocumentSizeKB float64
				}
				NITDocuments []struct {
					SerialNo       string
					DocumentName   string
					Description    string
					DocumentSizeKB float64
				}
			}{WorkItemDocuments: entry.WorkItemDocuments, NITDocuments: entry.NITDocuments})
		}

		if err := utils.AppendJSONL("out/tenders.jsonl", tender); err != nil {
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

// parseAmountToFloat converts strings like "1,40,000" or "13,30,00,000" to 140000.0 etc.
func parseAmountToFloat(amount string) float64 {
	clean := strings.TrimSpace(amount)
	if clean == "" || strings.EqualFold(clean, "na") {
		return 0
	}
	// remove commas and any non-numeric except dot
	r := regexp.MustCompile(`[^0-9\.]`)
	clean = r.ReplaceAllString(clean, "")
	v, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0
	}
	return v
}
