package extract

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/vx6fid/tender-scraper/session"
	"github.com/vx6fid/tender-scraper/utils"
)

type TenderDataScraper struct {
	collector        *colly.Collector
	activeTendersURL string
	// csvManager       *CSVManager
	parser  *TenderParser
	state   string
	runDate string
}

func NewTenderDataScraper(sess *session.Session, domain string, state string, runDate string) *TenderDataScraper {
	collector := sess.NewCollector(domain)

	return &TenderDataScraper{
		collector:        collector,
		activeTendersURL: sess.ActiveTendersURL,
		// csvManager:       NewCSVManager(),
		parser:  NewTenderParser(),
		state:   state,
		runDate: runDate,
	}
}

func (ts *TenderDataScraper) ExtractTenderData() error {
	log.Println("Starting tender scraping process with correct session flow.")

	// Load input CSV
	rows, err := ts.loadInputCSV()
	if err != nil {
		return err
	}

	// Channels
	jobs := make(chan TenderInput)
	results := make(chan *TenderData)
	var wg sync.WaitGroup
	var writerWg sync.WaitGroup

	// Writer goroutine (Single writer, avoid file race conditions)
	writerWg.Add(1)
	go func() {
		defer writerWg.Done()
		for tenderData := range results {
			// fmt.Printf("[%s] Writing tender data: %s\n", ts.state, tenderData.BasicDetails.TenderID)
			if err := ts.writeOutputs(tenderData); err != nil {
				log.Printf("[%s] failed to write outputs: %v", ts.state, err)
			}
		}
	}()

	// Worker pool
	maxWorkers := 10
	for range maxWorkers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for tenderInput := range jobs {
				// log.Printf("[%s] Worker %d processing tender: %s", ts.state, workerID, tenderInput.Title)
				tenderData, err := ts.extractSingleTender(tenderInput)
				if err != nil {
					log.Printf("[%s_%s] extraction failed: %v", ts.state, tenderInput.Serial, err)
					continue
				}

				// Only send to results if extraction was successful
				select {
				case results <- tenderData:
					log.Printf("[%s] Worker %d successfully sent tender %s to results channel", ts.state, workerID, tenderInput.Serial)
				case <-time.After(5 * time.Second):
					log.Printf("[%s] Worker %d timeout sending tender %s to results channel", ts.state, workerID, tenderInput.Serial)
				}
			}
		}(i)
	}

	// Enqueue jobs
	jobCount := 0
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) < 5 {
			continue
		}
		jobs <- TenderInput{
			Serial:       strings.TrimSpace(row[0]),
			Title:        strings.TrimSpace(row[1]),
			Organisation: strings.TrimSpace(row[2]),
			ClosingDate:  strings.TrimSpace(row[3]),
			Link:         strings.TrimSpace(row[4]),
		}
		jobCount++
	}
	close(jobs)
	log.Printf("[%s] Enqueued %d jobs", ts.state, jobCount)

	// Wait for all workers to complete, then close results channel
	go func() {
		wg.Wait()
		close(results)
		log.Printf("[%s] All workers completed, results channel closed", ts.state)
	}()

	// Wait for writer to finish processing all results
	writerWg.Wait()

	log.Printf("[%s] Tender data extraction completed.\n", ts.state)
	return nil
}

func (ts *TenderDataScraper) loadInputCSV() ([][]string, error) {
	fileName := fmt.Sprintf("%s_Links.csv", ts.state)
	filePath := fmt.Sprintf("TenderData/Links/%s", ts.runDate)
	// fmt.Println("FilePath: ", filePath)
	inputPath := filepath.Join(filePath, fileName)
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

func (ts *TenderDataScraper) writeOutputs(data *TenderData) error {
	// Write to JSONL
	tender := ts.convertToUtilsTender(data)
	dir := filepath.Join("TenderData/Tenders", ts.runDate)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fileName := filepath.Join(dir, "tender.jsonl")
	if err := utils.AppendJSONL(fileName, tender); err != nil {
		return fmt.Errorf("failed to append JSONL: %w", err)
	}

	log.Printf("[%s] wrote tender %s to %s\n", ts.state, data.BasicDetails.TenderID, fileName)
	return nil
}

func (ts *TenderDataScraper) convertToUtilsTender(data *TenderData) utils.Tender {
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
	// NEED FIX: Currently pushing all info onto OFFLINE category
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
	tender.Corrigenda = data.Corrigendum

	return tender
}
