package pastTenders

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/vx6fid/tender-scraper/session"
	"github.com/vx6fid/tender-scraper/utils"
	types "github.com/vx6fid/tender-scraper/utils/types"
)

func Run(dir string, runDate string, stage string) error {
	sessionLimiter := make(chan struct{}, utils.MaxSessionParallel)

	writeCh := make(chan *types.PastTenders) // global writer channel
	var writeWg sync.WaitGroup

	// Writer goroutine (sequential writes)
	writeWg.Add(1)
	go func() {
		defer writeWg.Done()
		for tender := range writeCh {
			if err := WriteAllTendersJSONL(tender, runDate); err != nil {
				log.Printf("failed to write tender: %v", err)
			}
		}
	}()

	// Loop over states
	for _, u := range utils.BaseURLs {
		fileName := fmt.Sprintf("%s_%s.csv", u.State, utils.StageName[stage])
		filePath := fmt.Sprintf("%s/%s", dir, fileName)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			continue
		}

		file, err := os.Open(filePath)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		reader := csv.NewReader(file)
		_, _ = reader.Read() // skip header

		// Prepare channel for this state
		recordsCh := make(chan []string, 1000)

		var wg sync.WaitGroup

		totalJobs, err := utils.EstimateJobCount(u.State, runDate, true, utils.StageName[stage])
		if totalJobs < 1 {
			log.Printf("No jobs found for state %s", u.State)
			continue
		}
		numWorkers := utils.CalculateOptimalWorkers(totalJobs)

		fmt.Print("\n\n")
		log.Printf("[%d] workers for state %s", numWorkers, u.State)
		// Launch workers for this state
		for i := range numWorkers {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				// log.Printf("[Worker-%d] started for state %s", workerID, u.State)

				// One session for this worker
				workerSess := session.NewSession(u.BaseURL, u.State)
				// One session for this worker
				sessionLimiter <- struct{}{}
				// start := time.Now()
				err := workerSess.EstablishTenderStatusSession(u.State, stage, "", "")
				if err != nil {
					<-sessionLimiter // release slot on failure
					log.Printf("[Worker-%d][%s] failed to establish worker session: %v", workerID, u.State, err)
					return
				}
				<-sessionLimiter

				// log.Printf("[Worker-%d][%s] worker session established in %v", workerID, u.State, time.Since(start))

				// Process all tenders with this session
				for record := range recordsCh {
					urlSnippet := record[6]

					tenderData := &TenderData{}
					tenderData.Website = u.BaseURL
					tenderData.TenderURL = urlSnippet
					tenderData.UniqueIdentifier = record[7]

					pastTenderData := &PastTendersData{}
					pasTenderExtractor := NewPastTender(workerSess, urlSnippet, u.Domain)
					if err := pasTenderExtractor.Extract(tenderData, pastTenderData); err != nil {
						log.Printf("[Worker-%d][%s] extraction failed for %s: %v", workerID, u.State, urlSnippet, err)
						continue
					}

					if pasTenderExtractor.validateTenderData(tenderData, pastTenderData) {
						tender := pasTenderExtractor.ConvertToUtilsTender(tenderData, pastTenderData)
						tender.LatestStage = utils.StageName[stage]
						tender.TenderInfo.UpdatedAt = time.Now()
						writeCh <- &tender
						// log.Printf("[Worker-%d][%s] tender written for %s", workerID, u.State, urlSnippet)
					} else {
						log.Printf("[Worker-%d][%s] tender validation failed for %s", workerID, u.State, urlSnippet)
					}
				}

				// log.Printf("[Worker-%d] finished for state %s", workerID, u.State)
			}(i)

		}

		// Feed records into this state’s channel
		for {
			record, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatal(err)
			}
			recordsCh <- record
		}

		close(recordsCh) // no more records for this state
		wg.Wait()        // wait for all workers of this state to finish
	}

	close(writeCh) // all states finished, stop writer
	writeWg.Wait() // wait for writer to finish
	fmt.Println()
	log.Println("All states finished")

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
func (ps *PastTender) ConvertToUtilsTender(data *TenderData, pastTenderData *PastTendersData) types.PastTenders {
	tender := types.PastTenders{}

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
	tender.TenderInfo.UniqueIdentifier = data.UniqueIdentifier

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
	tender.TenderInfo.Website = data.Website
	tender.TenderInfo.Link = data.TenderURL
	tender.TenderInfo.UpdatedAt = time.Now()

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
	// log.Printf("[MAPPING] Converting tender data - Bids: %d, Financial: %d, Awarded: %d",
	// 	len(pastTenderData.Bids),
	// 	len(pastTenderData.FinancialEvaluationBidList),
	// 	len(pastTenderData.AwardedBidsList))

	tender.BidsList = pastTenderData.Bids
	tender.StageUpdates = pastTenderData.StageUpdates
	// Extract the latest stage date from stage updates
	// tender.LatestStageDate = updateLatestStageDate(&tender.StageUpdates)
	tender.ContractValue = pastTenderData.ContractValue

	tender.FinancialEvaluationBidList = pastTenderData.FinancialEvaluationBidList
	tender.AwardedBidsList = pastTenderData.AwardedBidsList

	return tender
}
