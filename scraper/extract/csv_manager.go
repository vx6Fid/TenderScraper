package extract

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vx6fid/tender-scraper/utils"
)

type CSVManager struct{}

func NewCSVManager() *CSVManager {
	return &CSVManager{}
}

// CSVSinks groups structured CSV writers
type CSVSinks struct {
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

func (cm *CSVManager) WriteMainHeader(writer *csv.Writer) error {
	return writer.Write([]string{
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
	})
}

func (cm *CSVManager) WriteMainRow(writer *csv.Writer, input TenderInput, data *TenderData) error {
	row := []string{
		input.Serial,
		input.Title,
		input.Organisation,
		input.ClosingDate,
		data.DetailsURL,
		data.BasicDetails.TenderID,
		// Basic Details
		data.BasicDetails.OrganisationChain,
		data.BasicDetails.TenderReferenceNumber,
		data.BasicDetails.WithdrawalAllowed,
		data.BasicDetails.TenderType,
		data.BasicDetails.FormOfContract,
		data.BasicDetails.TenderCategory,
		data.BasicDetails.NumberOfCovers,
		data.BasicDetails.GeneralTechnicalEvaluationAllowed,
		data.BasicDetails.ItemWiseTechnicalEvaluationAllowed,
		data.BasicDetails.PaymentMode,
		data.BasicDetails.IsMultiCurrencyAllowedForBOQ,
		data.BasicDetails.IsMultiCurrencyAllowedForFee,
		data.BasicDetails.AllowTwoStageBidding,
	}

	if err := writer.Write(row); err != nil {
		return err
	}
	writer.Flush()
	return nil
}

// SetupStructuredCSVs creates files under out/ and writes headers
func (cm *CSVManager) SetupStructuredCSVs() (*CSVSinks, func(), error) {
	if err := os.MkdirAll("out", 0755); err != nil {
		return nil, nil, fmt.Errorf("failed to create out directory: %w", err)
	}

	files := make([]*os.File, 0, 9)
	writers := make([]*csv.Writer, 0, 9)

	// Helper to create file and writer
	createFile := func(filename string) (*os.File, *csv.Writer, error) {
		file, err := os.Create(filepath.Join("out", filename))
		if err != nil {
			return nil, nil, err
		}
		writer := csv.NewWriter(file)
		files = append(files, file)
		writers = append(writers, writer)
		return file, writer, nil
	}

	// Create all files
	_, basicW, err := createFile("basic_details.csv")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create basic_details.csv: %w", err)
	}

	_, payW, err := createFile("payment_instruments.csv")
	if err != nil {
		cm.closeFiles(files)
		return nil, nil, fmt.Errorf("failed to create payment_instruments.csv: %w", err)
	}

	_, coversW, err := createFile("cover_details.csv")
	if err != nil {
		cm.closeFiles(files)
		return nil, nil, fmt.Errorf("failed to create cover_details.csv: %w", err)
	}

	_, feeW, err := createFile("tender_fee_details.csv")
	if err != nil {
		cm.closeFiles(files)
		return nil, nil, fmt.Errorf("failed to create tender_fee_details.csv: %w", err)
	}

	_, emdW, err := createFile("emd_fee_details.csv")
	if err != nil {
		cm.closeFiles(files)
		return nil, nil, fmt.Errorf("failed to create emd_fee_details.csv: %w", err)
	}

	_, workW, err := createFile("work_item_details.csv")
	if err != nil {
		cm.closeFiles(files)
		return nil, nil, fmt.Errorf("failed to create work_item_details.csv: %w", err)
	}

	_, critW, err := createFile("critical_dates.csv")
	if err != nil {
		cm.closeFiles(files)
		return nil, nil, fmt.Errorf("failed to create critical_dates.csv: %w", err)
	}

	_, docsNitW, err := createFile("tender_documents_nit.csv")
	if err != nil {
		cm.closeFiles(files)
		return nil, nil, fmt.Errorf("failed to create tender_documents_nit.csv: %w", err)
	}

	_, docsWorkW, err := createFile("tender_documents_workitem.csv")
	if err != nil {
		cm.closeFiles(files)
		return nil, nil, fmt.Errorf("failed to create tender_documents_workitem.csv: %w", err)
	}

	// Write headers
	if err := cm.writeHeaders(basicW, payW, coversW, feeW, emdW, workW, critW, docsNitW, docsWorkW); err != nil {
		cm.closeFiles(files)
		return nil, nil, err
	}

	cleanup := func() {
		for _, writer := range writers {
			writer.Flush()
		}
		cm.closeFiles(files)
	}

	sinks := &CSVSinks{
		basicW:    basicW,
		payW:      payW,
		coversW:   coversW,
		feeW:      feeW,
		emdW:      emdW,
		workW:     workW,
		critW:     critW,
		docsNitW:  docsNitW,
		docsWorkW: docsWorkW,
	}

	return sinks, cleanup, nil
}

func (cm *CSVManager) closeFiles(files []*os.File) {
	for _, file := range files {
		file.Close()
	}
}

func (cm *CSVManager) writeHeaders(basicW, payW, coversW, feeW, emdW, workW, critW, docsNitW, docsWorkW *csv.Writer) error {
	headers := map[string][]string{
		"basic": {
			"Serial Number", "Title", "Organisation", "Closing Date", "Details URL", "Tender ID",
			"Organisation Chain", "Tender Reference Number", "Withdrawal Allowed", "Tender Type",
			"Form Of Contract", "Tender Category", "Number Of Covers", "General Technical Evaluation Allowed",
			"ItemWise Technical Evaluation Allowed", "Payment Mode", "Is Multi Currency Allowed For BOQ",
			"Is Multi Currency Allowed For Fee", "Allow Two Stage Bidding",
		},
		"payment": {
			"Serial Number", "Tender ID", "Instrument Mode", "S.No", "Instrument Type",
		},
		"covers": {
			"Serial Number", "Tender ID", "Cover No", "Cover", "Document Type", "Description",
		},
		"fee": {
			"Serial Number", "Tender ID", "Total Fee", "Tender Fee", "Fee Payable To",
			"Fee Payable At", "Tender Fee Exemption Allowed",
		},
		"emd": {
			"Serial Number", "Tender ID", "EMD Amount", "EMD Exemption Allowed", "EMD Fee Type",
			"EMD Percentage", "EMD Payable To", "EMD Payable At",
		},
		"work": {
			"Serial Number", "Tender ID", "Title", "Description", "PreQualification",
			"IndependentExternalMonitorRemarks", "TenderValue", "ProductCategory", "SubCategory",
			"ContractType", "BidValidityDays", "PeriodOfWorkDays", "Location", "Pincode",
			"PreBidMeetingPlace", "PreBidMeetingAddress", "PreBidMeetingDate", "BidOpeningPlace",
			"ShouldAllowNDATender", "AllowPreferentialBidder",
		},
		"critical": {
			"Serial Number", "Tender ID", "PublishedDate", "BidOpeningDate",
			"DocumentDownloadStartDate", "DocumentDownloadEndDate", "ClarificationStartDate",
			"ClarificationEndDate", "BidSubmissionStartDate", "BidSubmissionEndDate",
		},
		"docsNit": {
			"Serial Number", "Tender ID", "S.No", "Document Name", "Description", "Document Size (KB)",
		},
		"docsWork": {
			"Serial Number", "Tender ID", "S.No", "Document Type", "Document Name", "Description", "Document Size (KB)",
		},
	}

	writers := map[string]*csv.Writer{
		"basic":    basicW,
		"payment":  payW,
		"covers":   coversW,
		"fee":      feeW,
		"emd":      emdW,
		"work":     workW,
		"critical": critW,
		"docsNit":  docsNitW,
		"docsWork": docsWorkW,
	}

	for key, writer := range writers {
		if err := writer.Write(headers[key]); err != nil {
			return fmt.Errorf("failed to write %s header: %w", key, err)
		}
	}

	return nil
}

func (cm *CSVManager) WriteStructuredCSVs(sinks *CSVSinks, input TenderInput, data *TenderData) {
	serial := input.Serial
	tenderID := data.BasicDetails.TenderID

	// Write basic details
	cm.writeBasicDetails(sinks.basicW, serial, input.Title, input.Organisation, input.ClosingDate,
		data.DetailsURL, tenderID, &data.BasicDetails)

	// Write payment instruments
	cm.writePaymentInstruments(sinks.payW, serial, tenderID, "Offline", data.PaymentInstruments.Offline)
	cm.writePaymentInstruments(sinks.payW, serial, tenderID, "Online", data.PaymentInstruments.Online)

	// Write cover details
	cm.writeCoverDetails(sinks.coversW, serial, tenderID, data.Covers)

	// Write tender fee
	cm.writeTenderFee(sinks.feeW, serial, tenderID, &data.TenderFee)

	// Write EMD fee
	cm.writeEMDFee(sinks.emdW, serial, tenderID, &data.EMDFee)

	// Write work item
	cm.writeWorkItem(sinks.workW, serial, tenderID, &data.WorkItem)

	// Write critical dates
	cm.writeCriticalDates(sinks.critW, serial, tenderID, &data.CriticalDates)

	// Write documents
	cm.writeNITDocs(sinks.docsNitW, serial, tenderID, data.NITDocuments)
	cm.writeWorkItemDocs(sinks.docsWorkW, serial, tenderID, data.WorkDocuments)
}

// Helper methods for writing specific CSV sections
func (cm *CSVManager) writeBasicDetails(writer *csv.Writer, serial, title, organisation, closingDate, detailsURL, tenderID string, basic *BasicDetails) {
	row := []string{serial, title, organisation, closingDate, detailsURL, tenderID,
		basic.OrganisationChain, basic.TenderReferenceNumber, basic.WithdrawalAllowed,
		basic.TenderType, basic.FormOfContract, basic.TenderCategory, basic.NumberOfCovers,
		basic.GeneralTechnicalEvaluationAllowed, basic.ItemWiseTechnicalEvaluationAllowed,
		basic.PaymentMode, basic.IsMultiCurrencyAllowedForBOQ, basic.IsMultiCurrencyAllowedForFee,
		basic.AllowTwoStageBidding}
	writer.Write(row)
	writer.Flush()
}

func (cm *CSVManager) writePaymentInstruments(writer *csv.Writer, serial, tenderID, mode string, items []utils.PaymentInstrument) {
	for _, pi := range items {
		writer.Write([]string{serial, tenderID, mode, pi.SerialNo, pi.InstrumentType})
	}
	writer.Flush()
}

func (cm *CSVManager) writeCoverDetails(writer *csv.Writer, serial, tenderID string, covers []utils.CoverInformation) {
	for _, cv := range covers {
		writer.Write([]string{serial, tenderID, cv.CoverNo, cv.CoverType, cv.DocumentType, cv.Description})
	}
	writer.Flush()
}

func (cm *CSVManager) writeTenderFee(writer *csv.Writer, serial, tenderID string, fee *TenderFeeDetails) {
	writer.Write([]string{serial, tenderID, fee.TotalFee, fee.TenderFee, fee.FeePayableTo, fee.FeePayableAt, fee.TenderFeeExemptionAllowed})
	writer.Flush()
}

func (cm *CSVManager) writeEMDFee(writer *csv.Writer, serial, tenderID string, emd *EMDFeeDetails) {
	writer.Write([]string{serial, tenderID, emd.EmdAmount, emd.EmdExemptionAllowed, emd.EmdFeeType, emd.EmdPercentage, emd.EmdPayableTo, emd.EmdPayableAt})
	writer.Flush()
}

func (cm *CSVManager) writeWorkItem(writer *csv.Writer, serial, tenderID string, work *WorkItemDetails) {
	writer.Write([]string{serial, tenderID, work.Title, work.Description, work.PreQualification,
		work.IEMRemarks, work.TenderValue, work.ProductCategory, work.SubCategory, work.ContractType,
		work.BidValidityDays, work.PeriodOfWorkDays, work.Location, work.Pincode,
		work.PreBidMeetingPlace, work.PreBidMeetingAddress, work.PreBidMeetingDate, work.BidOpeningPlace,
		work.ShouldAllowNDA, work.AllowPreferentialBidder})
	writer.Flush()
}

func (cm *CSVManager) writeCriticalDates(writer *csv.Writer, serial, tenderID string, cd *CriticalDates) {
	val := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	writer.Write([]string{serial, tenderID, cd.PublishedDate, cd.BidOpeningDate,
		cd.DocumentDownloadStartDate, cd.DocumentDownloadEndDate,
		val(cd.ClarificationStartDate), val(cd.ClarificationEndDate),
		cd.BidSubmissionStartDate, cd.BidSubmissionEndDate})
	writer.Flush()
}

func (cm *CSVManager) writeNITDocs(writer *csv.Writer, serial, tenderID string, docs []NITDocument) {
	for _, d := range docs {
		writer.Write([]string{serial, tenderID, d.SerialNo, d.DocumentName, d.Description, d.DocumentSizeKB})
	}
	writer.Flush()
}

func (cm *CSVManager) writeWorkItemDocs(writer *csv.Writer, serial, tenderID string, docs []WorkDocument) {
	for _, d := range docs {
		writer.Write([]string{serial, tenderID, d.SerialNo, d.DocumentType, d.DocumentName, d.Description, d.DocumentSizeKB})
	}
	writer.Flush()
}
