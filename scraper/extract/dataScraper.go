package extract

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gocolly/colly/v2"
	"github.com/vx6fid/tender-scraper/session"
	"github.com/vx6fid/tender-scraper/utils"
)

type TenderDataScraper struct {
	collector        *colly.Collector
	activeTendersURL string
	csvManager       *CSVManager
	parser           *TenderParser
}

func NewTenderDataScraper(sess *session.Session, domain string, state string) *TenderDataScraper {
	collector := sess.NewCollector(domain)

	return &TenderDataScraper{
		collector:        collector,
		activeTendersURL: sess.ActiveTendersURL,
		csvManager:       NewCSVManager(),
		parser:           NewTenderParser(),
	}
}

func (ts *TenderDataScraper) ExtractTenderData() error {
	log.Println("Starting tender scraping process with correct session flow.")

	// Load input CSV
	rows, err := ts.loadInputCSV()
	if err != nil {
		return err
	}

	// Setup output files
	mainWriter, sinks, cleanup, err := ts.setupOutputFiles()
	if err != nil {
		return err
	}
	defer cleanup()

	// Process each tender
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) < 5 {
			continue
		}

		tenderInput := TenderInput{
			Serial:       strings.TrimSpace(row[0]),
			Title:        strings.TrimSpace(row[1]),
			Organisation: strings.TrimSpace(row[2]),
			ClosingDate:  strings.TrimSpace(row[3]),
			Link:         strings.TrimSpace(row[4]),
		}

		log.Printf("Processing tender %s: %s", tenderInput.Serial, tenderInput.Title)

		// Extract tender data
		tenderData, err := ts.extractSingleTender(tenderInput)
		if err != nil {
			log.Printf("[%s] extraction failed: %v", tenderInput.Serial, err)
			continue
		}

		// Write to all output formats
		if err := ts.writeOutputs(mainWriter, sinks, tenderInput, tenderData); err != nil {
			log.Printf("[%s] failed to write outputs: %v", tenderInput.Serial, err)
		}
	}

	log.Println("Tender data extraction completed.")
	return nil
}

func (ts *TenderDataScraper) loadInputCSV() ([][]string, error) {
	inputPath := filepath.Join("TenderData/Links", "MDL_Links.csv")
	inFile, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open links CSV at %s: %w", inputPath, err)
	}
	defer inFile.Close()

	reader := csv.NewReader(inFile)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read links CSV: %w", err)
	}
	if len(rows) <= 1 {
		return nil, fmt.Errorf("no data rows found in %s", inputPath)
	}

	return rows, nil
}

func (ts *TenderDataScraper) setupOutputFiles() (*csv.Writer, *CSVSinks, func(), error) {
	// Main CSV file
	outFile, err := os.Create("tenders.csv")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create output CSV: %w", err)
	}

	mainWriter := csv.NewWriter(outFile)
	if err := ts.csvManager.WriteMainHeader(mainWriter); err != nil {
		outFile.Close()
		return nil, nil, nil, err
	}

	// Structured CSV files
	sinks, sinkCleanup, err := ts.csvManager.SetupStructuredCSVs()
	if err != nil {
		outFile.Close()
		return nil, nil, nil, err
	}

	cleanup := func() {
		mainWriter.Flush()
		sinkCleanup()
		outFile.Close()
	}

	return mainWriter, sinks, cleanup, nil
}

func (ts *TenderDataScraper) extractSingleTender(input TenderInput) (*TenderData, error) {
	c := ts.collector.Clone()
	cookies := ts.collector.Cookies(ts.activeTendersURL)
	if len(cookies) > 0 {
		c.SetCookies(ts.activeTendersURL, cookies)
	}

	// Initialize tender data
	tenderData := &TenderData{}

	// Setup parser handlers
	ts.parser.SetupHandlers(c, tenderData)

	// Visit the tender page
	if err := c.Visit(input.Link); err != nil {
		return nil, fmt.Errorf("visit failed: %w", err)
	}

	return tenderData, nil
}

func (ts *TenderDataScraper) writeOutputs(mainWriter *csv.Writer, sinks *CSVSinks, input TenderInput, data *TenderData) error {
	// Write to main CSV
	if err := ts.csvManager.WriteMainRow(mainWriter, input, data); err != nil {
		return fmt.Errorf("failed to write main CSV: %w", err)
	}

	// Write to structured CSVs
	ts.csvManager.WriteStructuredCSVs(sinks, input, data)

	// Write to JSONL
	tender := ts.convertToUtilsTender(input, data)
	if err := utils.AppendJSONL("out/tenders.jsonl", tender); err != nil {
		return fmt.Errorf("failed to append JSONL: %w", err)
	}

	return nil
}

func (ts *TenderDataScraper) convertToUtilsTender(input TenderInput, data *TenderData) utils.Tender {
	tender := utils.Tender{}

	// Map basic details
	tender.BasicDetails.OrganisationChain = data.BasicDetails.OrganisationChain
	tender.BasicDetails.TenderReferenceNumber = data.BasicDetails.TenderReferenceNumber
	tender.BasicDetails.TenderID = data.BasicDetails.TenderID
	tender.BasicDetails.WithdrawalAllowed = strings.EqualFold(strings.TrimSpace(data.BasicDetails.WithdrawalAllowed), "yes")
	tender.BasicDetails.TenderType = data.BasicDetails.TenderType
	tender.BasicDetails.FormOfContract = data.BasicDetails.FormOfContract
	tender.BasicDetails.TenderCategory = data.BasicDetails.TenderCategory

	if n := strings.TrimSpace(data.BasicDetails.NumberOfCovers); n != "" {
		switch n {
		case "1":
			tender.BasicDetails.NumberOfCovers = 1
		case "2":
			tender.BasicDetails.NumberOfCovers = 2
		}
	}

	tender.BasicDetails.GeneralTechnicalEvaluationAllowed = strings.EqualFold(strings.TrimSpace(data.BasicDetails.GeneralTechnicalEvaluationAllowed), "yes")
	tender.BasicDetails.ItemWiseTechnicalEvaluationAllowed = strings.EqualFold(strings.TrimSpace(data.BasicDetails.ItemWiseTechnicalEvaluationAllowed), "yes")
	tender.BasicDetails.PaymentMode = data.BasicDetails.PaymentMode
	tender.BasicDetails.IsMultiCurrencyAllowedForBOQ = strings.EqualFold(strings.TrimSpace(data.BasicDetails.IsMultiCurrencyAllowedForBOQ), "yes")
	tender.BasicDetails.IsMultiCurrencyAllowedForFee = strings.EqualFold(strings.TrimSpace(data.BasicDetails.IsMultiCurrencyAllowedForFee), "yes")
	tender.BasicDetails.AllowTwoStageBidding = strings.EqualFold(strings.TrimSpace(data.BasicDetails.AllowTwoStageBidding), "yes")

	// Map other sections
	tender.PaymentInstruments.Offline = data.PaymentInstruments
	tender.CoversInformation = data.Covers

	// Map fee details
	tender.TenderFeeDetails.TotalFee = parseAmountToFloat(data.TenderFee.TotalFee)
	tender.TenderFeeDetails.TenderFee = parseAmountToFloat(data.TenderFee.TenderFee)
	tender.TenderFeeDetails.FeePayableTo = data.TenderFee.FeePayableTo
	tender.TenderFeeDetails.FeePayableAt = data.TenderFee.FeePayableAt
	tender.TenderFeeDetails.TenderFeeExemptionAllowed = strings.EqualFold(strings.TrimSpace(data.TenderFee.TenderFeeExemptionAllowed), "yes")

	// Map EMD details
	tender.EmdFeeDetails.EmdAmount = parseAmountToFloat(data.EMDFee.EmdAmount)
	tender.EmdFeeDetails.EmdExemptionAllowed = strings.EqualFold(strings.TrimSpace(data.EMDFee.EmdExemptionAllowed), "yes")
	tender.EmdFeeDetails.EmdFeeType = data.EMDFee.EmdFeeType
	if strings.TrimSpace(data.EMDFee.EmdPercentage) != "" && !strings.EqualFold(strings.TrimSpace(data.EMDFee.EmdPercentage), "na") {
		pct := strings.TrimSpace(data.EMDFee.EmdPercentage)
		tender.EmdFeeDetails.EmdPercentage = &pct
	}
	tender.EmdFeeDetails.EmdPayableTo = data.EMDFee.EmdPayableTo
	tender.EmdFeeDetails.EmdPayableAt = data.EMDFee.EmdPayableAt

	// Map work item details
	ts.mapWorkItemDetails(&tender, data)

	// Map critical dates
	tender.CriticalDates = data.CriticalDates

	// Map tender documents
	ts.mapTenderDocuments(&tender, data)

	// Map Tender Inviting Authority
	tender.TenderInvitingAuthority.Name = data.TenderInvitingAuthority.Name
	tender.TenderInvitingAuthority.Address = data.TenderInvitingAuthority.Address

	return tender
}
