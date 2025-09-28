package pastTenders

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/vx6fid/tender-scraper/session"
	"github.com/vx6fid/tender-scraper/utils"
)

func Run(dir string, runDate string) error {
	for _, u := range utils.BaseURLs {
		fileName := u.State + ".csv"
		filePath := fmt.Sprintf("%s/%s", dir, fileName)
		// If there is no file, skip it
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			continue
		}
		// Read each row of the above file, skip the header
		file, err := os.Open(filePath)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		reader := csv.NewReader(file)
		reader.Read() // Skip header

		for {
			record, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatal(err)
			}
			sess := session.NewSession(u.BaseURL, u.State)
			if err := sess.EstablishTenderStatusSession("6", "", ""); err != nil {
				log.Printf("[%s] failed to establish session: %v", u.State, err)
			}
			tenderData := &TenderData{}
			tenderData.Information.Website = u.Domain
			tenderData.Information.TenderURL = record[6]

			pastTenderData := &PastTendersData{}

			pasTenderExtractor := NewPastTender(sess, record[6], u.Domain)
			pasTenderExtractor.Extract(tenderData, pastTenderData)

			fmt.Println("Tender Data:", tenderData)
			fmt.Println("Past Tender Data:", pastTenderData)

			// Validate and convert before writing
			// if pasTenderExtractor.validateTenderData(tenderData, pastTenderData) {
			tender := pasTenderExtractor.ConvertToUtilsTender(tenderData, pastTenderData)
			if err := WriteAllTendersJSONL(tender, runDate); err != nil {
				log.Printf("[%s] failed to write tender: %v", u.State, err)
			}
			// }this

		}
	}
	return nil
}

// validateTenderData checks if meaningful data was extracted
func (ps *PastTender) validateTenderData(data *TenderData, pastTenderData *PastTendersData) bool {
	if data == nil || pastTenderData == nil {
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

	hasPastTenderData := len(pastTenderData.Bids) > 0 ||
		len(pastTenderData.FinancialEvaluationBidList) > 0 ||
		len(pastTenderData.AwardedBidsList) > 0

	return hasBasicDetails || hasCriticalDates || hasWorkItem || hasFeeDetails || hasPastTenderData
}

// ConvertToUtilsTender converts internal TenderData to utils.Tender
func (ps *PastTender) ConvertToUtilsTender(data *TenderData, pastTenderData *PastTendersData) utils.PastTenders {
	tender := utils.PastTenders{}

	//------------------
	// Mapping Tender Basic Details
	//------------------
	// Map basic details
	tender.TenderInfo.BasicDetails.OrganisationChain = data.BasicDetails.OrganisationChain
	tender.TenderInfo.BasicDetails.TenderReferenceNumber = data.BasicDetails.TenderReferenceNumber
	tender.TenderInfo.BasicDetails.TenderID = data.BasicDetails.TenderID
	tender.TenderInfo.BasicDetails.WithdrawalAllowed = strings.EqualFold(strings.TrimSpace(data.BasicDetails.WithdrawalAllowed), "yes")
	tender.TenderInfo.BasicDetails.TenderType = data.BasicDetails.TenderType
	tender.TenderInfo.BasicDetails.FormOfContract = data.BasicDetails.FormOfContract
	tender.TenderInfo.BasicDetails.TenderCategory = data.BasicDetails.TenderCategory

	// Map number of covers
	if n := strings.TrimSpace(data.BasicDetails.NumberOfCovers); n != "" {
		switch n {
		case "1":
			tender.TenderInfo.BasicDetails.NumberOfCovers = 1
		case "2":
			tender.TenderInfo.BasicDetails.NumberOfCovers = 2
		}
	}

	tender.TenderInfo.BasicDetails.GeneralTechnicalEvaluationAllowed = strings.EqualFold(strings.TrimSpace(data.BasicDetails.GeneralTechnicalEvaluationAllowed), "yes")
	tender.TenderInfo.BasicDetails.ItemWiseTechnicalEvaluationAllowed = strings.EqualFold(strings.TrimSpace(data.BasicDetails.ItemWiseTechnicalEvaluationAllowed), "yes")
	tender.TenderInfo.BasicDetails.PaymentMode = data.BasicDetails.PaymentMode
	tender.TenderInfo.BasicDetails.IsMultiCurrencyAllowedForBOQ = strings.EqualFold(strings.TrimSpace(data.BasicDetails.IsMultiCurrencyAllowedForBOQ), "yes")
	tender.TenderInfo.BasicDetails.IsMultiCurrencyAllowedForFee = strings.EqualFold(strings.TrimSpace(data.BasicDetails.IsMultiCurrencyAllowedForFee), "yes")
	tender.TenderInfo.BasicDetails.AllowTwoStageBidding = strings.EqualFold(strings.TrimSpace(data.BasicDetails.AllowTwoStageBidding), "yes")

	// Information Section
	tender.TenderInfo.Information.Website = data.Information.Website
	tender.TenderInfo.Information.Link = data.Information.TenderURL
	tender.TenderInfo.Information.UpdatedAt = time.Now()

	// Map other sections
	tender.TenderInfo.PaymentInstruments.Online = data.PaymentInstruments.Online
	tender.TenderInfo.PaymentInstruments.Offline = data.PaymentInstruments.Offline
	tender.TenderInfo.CoversInformation = data.Covers

	// Map fee details
	tender.TenderInfo.TenderFeeDetails.TotalFee = parseAmountToFloat(data.TenderFee.TotalFee)
	tender.TenderInfo.TenderFeeDetails.TenderFee = parseAmountToFloat(data.TenderFee.TenderFee)
	tender.TenderInfo.TenderFeeDetails.FeePayableTo = data.TenderFee.FeePayableTo
	tender.TenderInfo.TenderFeeDetails.FeePayableAt = data.TenderFee.FeePayableAt
	tender.TenderInfo.TenderFeeDetails.TenderFeeExemptionAllowed = strings.EqualFold(strings.TrimSpace(data.TenderFee.TenderFeeExemptionAllowed), "yes")

	// Map EMD details
	tender.TenderInfo.EmdFeeDetails.EmdAmount = parseAmountToFloat(data.EMDFee.EmdAmount)
	tender.TenderInfo.EmdFeeDetails.EmdExemptionAllowed = strings.EqualFold(strings.TrimSpace(data.EMDFee.EmdExemptionAllowed), "yes")
	tender.TenderInfo.EmdFeeDetails.EmdFeeType = data.EMDFee.EmdFeeType
	if strings.TrimSpace(data.EMDFee.EmdPercentage) != "" && !strings.EqualFold(strings.TrimSpace(data.EMDFee.EmdPercentage), "na") {
		pct := strings.TrimSpace(data.EMDFee.EmdPercentage)
		tender.TenderInfo.EmdFeeDetails.EmdPercentage = &pct
	}
	tender.TenderInfo.EmdFeeDetails.EmdPayableTo = data.EMDFee.EmdPayableTo
	tender.TenderInfo.EmdFeeDetails.EmdPayableAt = data.EMDFee.EmdPayableAt

	// Map work item details
	ps.mapWorkItemDetails(&tender.TenderInfo, data)

	// Map critical dates
	tender.TenderInfo.CriticalDates = data.CriticalDates

	// Map tender documents
	ps.mapTenderDocuments(&tender.TenderInfo, data)

	// Map Tender Inviting Authority
	tender.TenderInfo.TenderInvitingAuthority.Name = data.TenderInvitingAuthority.Name
	tender.TenderInfo.TenderInvitingAuthority.Address = data.TenderInvitingAuthority.Address
	tender.TenderInfo.Corrigenda = data.Corrigendum

	//------------------
	// Mapping Tender Summary Details
	//------------------
	log.Printf("[MAPPING] Converting tender data - Bids: %d, Financial: %d, Awarded: %d",
		len(pastTenderData.Bids),
		len(pastTenderData.FinancialEvaluationBidList),
		len(pastTenderData.AwardedBidsList))

	tender.Bids = pastTenderData.Bids
	tender.StageUpdates = pastTenderData.StageUpdates
	tender.FinancialEvaluationBidList = pastTenderData.FinancialEvaluationBidList
	tender.AwardedBidsList = pastTenderData.AwardedBidsList

	return tender
}
