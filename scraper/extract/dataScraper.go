package extract

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/vx6fid/tender-scraper/session"
	types "github.com/vx6fid/tender-scraper/utils/types"
)

// DataScraper handles individual tender data extraction with its own session
type DataScraper struct {
	// session   *session.Session
	baseURL   string
	domain    string
	state     string
	runDate   string
	collector *colly.Collector
}

func NewDataScraper(sess *session.Session, domain, state, runDate string) *DataScraper {
	c := sess.NewCollector(domain)

	return &DataScraper{
		baseURL:   sess.BaseURL,
		domain:    domain,
		state:     state,
		runDate:   runDate,
		collector: c,
	}
}

// ExtractSingleTender extracts data from a single tender URL
func (ds *DataScraper) ExtractSingleTender(input TenderInput) (*TenderData, error) {
	// Fresh TenderData
	tenderData := &TenderData{}

	tenderData.Website = ds.baseURL // or ds.session.BaseURL
	tenderData.TenderURL = input.Link

	// Setup parser handlers
	parser := NewTenderParser()
	c := ds.collector.Clone()
	parser.SetupHandlers(c, tenderData)

	start := time.Now()
	err := c.Visit(input.Link)

	if err == nil && ds.validateTenderData(tenderData) {
		log.Printf("[%s_%s] Successfully extracted tender in %v", ds.state, input.Serial, time.Since(start))
		return tenderData, nil
	}

	if err == nil {
		return nil, fmt.Errorf("no data extracted from %s", input.Link)
	}
	return nil, fmt.Errorf("scraper error for %s: %w", input.Link, err)
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
func (ds *DataScraper) ConvertToUtilsTender(data *TenderData) types.Tender {
	tender := types.Tender{}

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
		tender.BasicDetails.NumberOfCovers, _ = strconv.Atoi(n)
	}

	tender.BasicDetails.GeneralTechnicalEvaluationAllowed = strings.EqualFold(strings.TrimSpace(data.BasicDetails.GeneralTechnicalEvaluationAllowed), "yes")
	tender.BasicDetails.ItemWiseTechnicalEvaluationAllowed = strings.EqualFold(strings.TrimSpace(data.BasicDetails.ItemWiseTechnicalEvaluationAllowed), "yes")
	tender.BasicDetails.PaymentMode = data.BasicDetails.PaymentMode
	tender.BasicDetails.IsMultiCurrencyAllowedForBOQ = strings.EqualFold(strings.TrimSpace(data.BasicDetails.IsMultiCurrencyAllowedForBOQ), "yes")
	tender.BasicDetails.IsMultiCurrencyAllowedForFee = strings.EqualFold(strings.TrimSpace(data.BasicDetails.IsMultiCurrencyAllowedForFee), "yes")
	tender.BasicDetails.AllowTwoStageBidding = strings.EqualFold(strings.TrimSpace(data.BasicDetails.AllowTwoStageBidding), "yes")

	// Information Section
	tender.Website = data.Website
	tender.Link = data.TenderURL
	tender.UpdatedAt = time.Now()

	// Map other sections
	tender.PaymentInstruments.Online = data.PaymentInstruments.Online
	tender.PaymentInstruments.Offline = data.PaymentInstruments.Offline
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
