package extract

import "github.com/vx6fid/tender-scraper/utils"

// TenderInput represents the input data from CSV
type TenderInput struct {
	Serial       string
	Title        string
	Organisation string
	ClosingDate  string
	Link         string
}

// BasicDetails holds basic tender information
type BasicDetails struct {
	OrganisationChain                  string
	TenderReferenceNumber              string
	TenderID                           string
	WithdrawalAllowed                  string
	TenderType                         string
	FormOfContract                     string
	TenderCategory                     string
	NumberOfCovers                     string
	GeneralTechnicalEvaluationAllowed  string
	ItemWiseTechnicalEvaluationAllowed string
	PaymentMode                        string
	IsMultiCurrencyAllowedForBOQ       string
	IsMultiCurrencyAllowedForFee       string
	AllowTwoStageBidding               string
}

// TenderFeeDetails holds fee-related information
type TenderFeeDetails struct {
	TotalFee                  string
	TenderFee                 string
	FeePayableTo              string
	FeePayableAt              string
	TenderFeeExemptionAllowed string
}

// EMDFeeDetails holds EMD-related information
type EMDFeeDetails struct {
	EmdAmount           string
	EmdExemptionAllowed string
	EmdFeeType          string
	EmdPercentage       string
	EmdPayableTo        string
	EmdPayableAt        string
}

// WorkItemDetails holds work/item information
type WorkItemDetails struct {
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

// CriticalDates holds date information
type CriticalDates struct {
	PublishedDate             string
	BidOpeningDate            string
	DocumentDownloadStartDate string
	DocumentDownloadEndDate   string
	ClarificationStartDate    *string
	ClarificationEndDate      *string
	BidSubmissionStartDate    string
	BidSubmissionEndDate      string
}

// NITDocument represents a NIT document
type NITDocument struct {
	SerialNo       string
	DocumentName   string
	Description    string
	DocumentSizeKB string
}

// WorkDocument represents a work item document
type WorkDocument struct {
	SerialNo       string
	DocumentType   string
	DocumentName   string
	Description    string
	DocumentSizeKB string
}

// TenderData holds all extracted tender information
type TenderData struct {
	DetailsURL              string
	BasicDetails            BasicDetails
	PaymentInstruments      []utils.PaymentInstrument
	Covers                  []utils.CoverInformation
	TenderFee               TenderFeeDetails
	EMDFee                  EMDFeeDetails
	WorkItem                WorkItemDetails
	CriticalDates           CriticalDates
	NITDocuments            []NITDocument
	WorkDocuments           []WorkDocument
	TenderInvitingAuthority struct {
		Name    string
		Address string
	}
}
