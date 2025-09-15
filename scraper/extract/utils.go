package extract

import (
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/vx6fid/tender-scraper/utils"
)

// WriteJSONLToFile encodes v as JSON and appends it to an already-open file
func WriteJSONLToFile(file *os.File, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	// Append newline explicitly (JSONL = one JSON per line)
	_, err = file.Write(append(data, '\n'))
	return err
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

// Helper methods for mapping data structures
func (ds *DataScraper) mapWorkItemDetails(tender *utils.Tender, data *TenderData) {
	tender.WorkItemDetails.Title = data.WorkItem.Title
	tender.WorkItemDetails.Description = data.WorkItem.Description
	tender.WorkItemDetails.NDAOrPreQualification = data.WorkItem.PreQualification
	tender.WorkItemDetails.IndependentExternalMonitorRemarks = data.WorkItem.IEMRemarks
	tender.WorkItemDetails.TenderValue = parseAmountToFloat(data.WorkItem.TenderValue)
	tender.WorkItemDetails.ProductCategory = data.WorkItem.ProductCategory

	if ws := strings.TrimSpace(data.WorkItem.SubCategory); ws != "" && !strings.EqualFold(ws, "na") {
		tender.WorkItemDetails.SubCategory = &ws
	}

	tender.WorkItemDetails.ContractType = data.WorkItem.ContractType

	if d, err := strconv.Atoi(strings.TrimSpace(data.WorkItem.BidValidityDays)); err == nil {
		tender.WorkItemDetails.BidValidityDays = d
	}

	if pd := strings.TrimSpace(data.WorkItem.PeriodOfWorkDays); pd != "" && !strings.EqualFold(pd, "na") {
		if v, err := strconv.Atoi(pd); err == nil {
			tender.WorkItemDetails.PeriodOfWorkDays = &v
		}
	}

	tender.WorkItemDetails.Location = data.WorkItem.Location
	tender.WorkItemDetails.Pincode = data.WorkItem.Pincode

	if v := strings.TrimSpace(data.WorkItem.PreBidMeetingPlace); v != "" && !strings.EqualFold(v, "na") {
		tender.WorkItemDetails.PreBidMeetingPlace = &v
	}

	if v := strings.TrimSpace(data.WorkItem.PreBidMeetingAddress); v != "" && !strings.EqualFold(v, "na") {
		tender.WorkItemDetails.PreBidMeetingAddress = &v
	}

	if v := strings.TrimSpace(data.WorkItem.PreBidMeetingDate); v != "" && !strings.EqualFold(v, "na") {
		tender.WorkItemDetails.PreBidMeetingDate = &v
	}

	tender.WorkItemDetails.BidOpeningPlace = data.WorkItem.BidOpeningPlace
	tender.WorkItemDetails.ShouldAllowNDATender = strings.EqualFold(strings.TrimSpace(data.WorkItem.ShouldAllowNDA), "yes")
	tender.WorkItemDetails.AllowPreferentialBidder = strings.EqualFold(strings.TrimSpace(data.WorkItem.AllowPreferentialBidder), "yes")
}

func (ds *DataScraper) mapTenderDocuments(tender *utils.Tender, data *TenderData) {
	if len(data.NITDocuments) > 0 || len(data.WorkDocuments) > 0 {
		entry := struct {
			WorkItemLink      string
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
				Link           string
			}
		}{}

		for _, d := range data.WorkDocuments {
			entry.WorkItemDocuments = append(entry.WorkItemDocuments, struct {
				SerialNo       string
				DocumentType   string
				DocumentName   string
				Description    string
				DocumentSizeKB float64
			}{
				SerialNo:       d.SerialNo,
				DocumentType:   d.DocumentType,
				DocumentName:   d.DocumentName,
				Description:    d.Description,
				DocumentSizeKB: parseAmountToFloat(d.DocumentSizeKB),
			})
		}

		for _, d := range data.NITDocuments {
			entry.NITDocuments = append(entry.NITDocuments, struct {
				SerialNo       string
				DocumentName   string
				Description    string
				DocumentSizeKB float64
				Link           string
			}{
				SerialNo:       d.SerialNo,
				DocumentName:   d.DocumentName,
				Description:    d.Description,
				DocumentSizeKB: parseAmountToFloat(d.DocumentSizeKB),
				Link:           d.Link,
			})
		}

		// Append single combined entry to TenderDocuments slice
		if len(data.NITDocuments) > 0 || len(data.WorkDocuments) > 0 {

			// Convert WorkDocuments
			workDocs := make([]utils.WorkItemDocument, 0, len(data.WorkDocuments))
			for _, d := range data.WorkDocuments {
				workDocs = append(workDocs, utils.WorkItemDocument{
					SerialNo:       d.SerialNo,
					DocumentType:   d.DocumentType,
					DocumentName:   d.DocumentName,
					Description:    d.Description,
					DocumentSizeKB: parseAmountToFloat(d.DocumentSizeKB),
				})
			}

			// Convert NITDocuments
			nitDocs := make([]utils.NITDocument, 0, len(data.NITDocuments))
			for _, d := range data.NITDocuments {
				nitDocs = append(nitDocs, utils.NITDocument{
					SerialNo:       d.SerialNo,
					DocumentName:   d.DocumentName,
					Description:    d.Description,
					DocumentSizeKB: parseAmountToFloat(d.DocumentSizeKB),
					Link:           d.Link,
				})
			}

			// Assign directly to the TenderDocument field
			tender.TenderDocument.WorkItemLink = data.WorkItemLink // single link for all work item documents
			tender.TenderDocument.WorkItemDocuments = workDocs
			tender.TenderDocument.NITDocuments = nitDocs
		}

	}
}
