package utils

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

	PaymentInstruments struct {
		Offline struct {
			SerialNo       string
			InstrumentType string
		}
		Online struct {
			SerialNo       string
			InstrumentType string
		}
	}

	CoversInformation []struct {
		CoverNo      string
		CoverType    string
		Description  string
		DocumentType string
	}

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

	Corrigenda []struct {
		SerialNo string
		Title    string
		Type     string // Enum-like, but open-ended
	}

	TenderInvitingAuthority struct {
		Name    string
		Address string
	}
}

// Tender Links holds one scraped record
type TenderLinks struct {
	Serial       string
	Title        string
	Organisation string
	ClosingDate  string
	Link         string
}
