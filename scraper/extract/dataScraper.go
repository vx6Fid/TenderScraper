package extract

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/vx6fid/tender-scraper/session"
	"github.com/vx6fid/tender-scraper/utils"
)

// DataScraper handles individual tender data extraction with its own session
type DataScraper struct {
	session *session.Session
	domain  string
	state   string
	runDate string
}

func NewDataScraper(sess *session.Session, domain, state, runDate string) *DataScraper {
	return &DataScraper{
		session: sess,
		domain:  domain,
		state:   state,
		runDate: runDate,
	}
}

// ExtractSingleTender extracts data from a single tender URL
func (ds *DataScraper) ExtractSingleTender(input TenderInput) (*TenderData, error) {
	// Create completely fresh collector for this request to avoid state conflicts
	c := ds.session.NewCollector(ds.domain)

	// Configure collector with conservative settings
	c.SetRequestTimeout(60 * time.Second)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       500 * time.Millisecond, // Increased delay for stability
	})

	// Add realistic headers
	c.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"
	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.5")
		r.Headers.Set("Accept-Encoding", "gzip, deflate")
		r.Headers.Set("Connection", "keep-alive")
		r.Headers.Set("Upgrade-Insecure-Requests", "1")
		r.Headers.Set("Cache-Control", "no-cache")
	})

	// Add error handler for debugging
	c.OnError(func(r *colly.Response, err error) {
		log.Printf("[%s] Request error for %s: %v", ds.state, r.Request.URL, err)
	})

	// Create fresh TenderData for this extraction
	tenderData := &TenderData{}

	// Create a new parser instance for this extraction to avoid state conflicts
	parser := NewTenderParser()
	parser.SetupHandlers(c, tenderData)

	// Implement retry logic
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Reset tender data for retry attempts
		if attempt > 1 {
			tenderData = &TenderData{}
			// Create completely new collector for retry to avoid state issues
			c = ds.session.NewCollector(ds.domain)

			// Create new parser instance for retry
			parser = NewTenderParser()
			parser.SetupHandlers(c, tenderData)
		}

		// Create context for this attempt
		ctx := colly.NewContext()

		// Attempt the extraction with timeout
		visitCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

		done := make(chan error, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					done <- fmt.Errorf("panic during extraction: %v", r)
				}
			}()
			done <- c.Request("GET", input.Link, nil, ctx, nil)
		}()

		select {
		case err := <-done:
			cancel()
			if err == nil {
				// Success! Validate that we got some data
				if ds.validateTenderData(tenderData) {
					log.Printf("[%s_%s] Successfully extracted tender data", ds.state, input.Serial)
					return tenderData, nil
				} else {
					log.Printf("[%s_%s] Attempt %d: No data extracted, treating as failure",
						ds.state, input.Serial, attempt)
					lastErr = fmt.Errorf("no data extracted from %s", input.Link)
				}
			} else {
				lastErr = err
				log.Printf("[%s_%s] Attempt %d failed: %v", ds.state, input.Serial, attempt, err)
			}
		case <-visitCtx.Done():
			cancel()
			lastErr = fmt.Errorf("timeout for %s", input.Link)
			log.Printf("[%s_%s] Attempt %d timeout", ds.state, input.Serial, attempt)
		}

		// Wait before retry with exponential backoff
		if attempt < maxRetries {
			backoffDuration := time.Duration(attempt*attempt) * 3 * time.Second
			log.Printf("[%s_%s] Retrying in %v...", ds.state, input.Serial, backoffDuration)
			time.Sleep(backoffDuration)
		}
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

// validateTenderData checks if meaningful data was extracted
func (ds *DataScraper) validateTenderData(data *TenderData) bool {
	if data == nil {
		return false
	}

	// Check if we have at least some basic details or other meaningful data
	hasBasicDetails := data.BasicDetails.TenderID != "" ||
		data.BasicDetails.TenderReferenceNumber != "" ||
		data.BasicDetails.OrganisationChain != ""

	hasCriticalDates := data.CriticalDates.PublishedDate != "" ||
		data.CriticalDates.BidSubmissionEndDate != ""

	hasWorkItem := data.WorkItem.Title != "" ||
		data.WorkItem.Description != ""

	hasFeeDetails := data.TenderFee.TotalFee != "" ||
		data.EMDFee.EmdAmount != ""

	return hasBasicDetails || hasCriticalDates || hasWorkItem || hasFeeDetails
}

// ConvertToUtilsTender converts internal TenderData to utils.Tender
func (ds *DataScraper) ConvertToUtilsTender(data *TenderData) utils.Tender {
	tender := utils.Tender{}

	// Map basic details
	tender.BasicDetails.OrganisationChain = data.BasicDetails.OrganisationChain
	tender.BasicDetails.TenderReferenceNumber = data.BasicDetails.TenderReferenceNumber
	tender.BasicDetails.TenderID = data.BasicDetails.TenderID
	tender.BasicDetails.WithdrawalAllowed = strings.EqualFold(strings.TrimSpace(data.BasicDetails.WithdrawalAllowed), "yes")
	tender.BasicDetails.TenderType = data.BasicDetails.TenderType
	tender.BasicDetails.FormOfContract = data.BasicDetails.FormOfContract
	tender.BasicDetails.TenderCategory = data.BasicDetails.TenderCategory

	// Map number of covers
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
	ds.mapWorkItemDetails(&tender, data)

	// Map critical dates
	tender.CriticalDates = data.CriticalDates

	// Map tender documents
	ds.mapTenderDocuments(&tender, data)

	// Map Tender Inviting Authority
	tender.TenderInvitingAuthority.Name = data.TenderInvitingAuthority.Name
	tender.TenderInvitingAuthority.Address = data.TenderInvitingAuthority.Address
	tender.Corrigenda = data.Corrigendum

	return tender
}
