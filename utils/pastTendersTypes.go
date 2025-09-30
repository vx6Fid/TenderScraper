package utils

type PastTenders struct {
	TenderInfo                 Tender
	BidsList                   []Bid
	StageUpdates               StageUpdates
	FinancialEvaluationBidList []FinancialEvaluationBidList
	AwardedBidsList            []AwardedBidsList
	LatestStage                string
	ContractValue              float64
}

type Bid struct {
	SNo             int
	BidNumber       string
	BidderName      string
	SubmittedDate   string
	Status          string
	Remarks         string
	StatusUpdatedOn string
}

type FinancialEvaluationBidList struct {
	BidNumber  string
	BidderName string
	Value      float64
	Rank       string
}

type AwardedBidsList struct {
	BidNumber       string
	BidderName      string
	AwardedCurrency string
	AwardedValue    float64
}

type StageUpdates struct {
	TechnicalBidOpeningUpdatedOn string
	TechnicalEvaluationUpdatedOn string
	FinancialEvaluationUpdatedOn string
	AOCUpdatedOn                 string
}
