package utils

import "time"

type URLS struct {
	BaseURL string
	State   string
	Domain  string
}

type CorrigendumDetail struct {
	CorrigendumNo  string
	Title          string
	Description    string
	PublishedDate  string
	DocumentName   string
	DocumentLink   string
	DocumentSizeKB string
}

type Corrigendum struct {
	SerialNo string
	Title    string
	Type     string
	ViewLink string
	Details  []CorrigendumDetail
}

// Shared Types
type Tender struct {
	BasicDetails struct {
		OrganisationChain                  string
		TenderReferenceNumber              string
		TenderID                           string
		WithdrawalAllowed                  bool
		TenderType                         string
		FormOfContract                     string
		TenderCategory                     string
		NumberOfCovers                     int
		GeneralTechnicalEvaluationAllowed  bool
		ItemWiseTechnicalEvaluationAllowed bool
		PaymentMode                        string
		IsMultiCurrencyAllowedForBOQ       bool
		IsMultiCurrencyAllowedForFee       bool
		AllowTwoStageBidding               bool
	}

	Information struct {
		Website   string
		Link      string // tender link
		CreatedAt time.Time
		UpdatedAt time.Time
	}

	PaymentInstruments struct {
		Offline []PaymentInstrument
		Online  []PaymentInstrument
	}

	CoversInformation []CoverInformation

	TenderFeeDetails struct {
		TotalFee                  float64
		TenderFee                 float64
		ProcessingFee             float64
		FeePayableTo              string
		FeePayableAt              string
		TenderFeeExemptionAllowed bool
	}

	EmdFeeDetails struct {
		EmdAmount           float64
		EmdExemptionAllowed bool
		EmdFeeType          string
		EmdPercentage       *string // nullable in Go
		EmdPayableTo        string
		EmdPayableAt        string
	}

	WorkItemDetails struct {
		Title                             string
		Description                       string
		NDAOrPreQualification             string
		IndependentExternalMonitorRemarks string
		TenderValue                       float64
		ProductCategory                   string
		SubCategory                       *string
		ContractType                      string
		BidValidityDays                   int
		PeriodOfWorkDays                  *int
		Location                          string
		Pincode                           string
		PreBidMeetingPlace                *string
		PreBidMeetingAddress              *string
		PreBidMeetingDate                 *string
		BidOpeningPlace                   string
		ShouldAllowNDATender              bool
		AllowPreferentialBidder           bool
	}

	CriticalDates struct {
		PublishedDate             string
		BidOpeningDate            string
		DocumentDownloadStartDate string
		DocumentDownloadEndDate   string
		ClarificationStartDate    *string
		ClarificationEndDate      *string
		BidSubmissionStartDate    string
		BidSubmissionEndDate      string
	}

	TenderDocuments []struct {
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
	}

	Corrigenda []Corrigendum

	TenderInvitingAuthority struct {
		Name    string
		Address string
	}
}

type PaymentInstrument struct {
	SerialNo       string
	InstrumentType string
}

type CoverInformation struct {
	CoverNo      string
	CoverType    string
	Description  string
	DocumentType string
}

// Tender Links holds one scraped record
type TenderLinks struct {
	Serial       string
	Title        string
	Organisation string
	ClosingDate  string
	Link         string
}
