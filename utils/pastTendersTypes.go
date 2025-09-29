package utils

import "time"

type PastTenders struct {
	TenderInfo                 Tender
	BidsList                   []Bid
	StageUpdates               StageUpdates
	FinancialEvaluationBidList []FinancialEvaluationBidList
	AwardedBidsList            []AwardedBidsList
	LatestStage                string
	LatestStageDate            string
	ContractValue              string
}

type Bid struct {
	SNo             int
	BidNumber       string
	BidderName      string
	SubmittedDate   time.Time
	Status          string
	Remarks         string
	StatusUpdatedOn time.Time
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
