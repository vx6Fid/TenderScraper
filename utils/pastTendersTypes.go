package utils

import "time"

type PastTenders struct {
	TenderInfo                 Tender
	BidList                    []Bid
	StageUpdates               StageUpdates
	FinancialEvaluationBidList []FinancialEvaluationBidList
	AwardedBidsList            []AwardedBidsList
	LatestStage                string
	LatestStageDate            time.Time
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
	TechnicalBidOpeningUpdatedOn time.Time
	TechnicalEvaluationUpdatedOn time.Time
	FinancialEvaluationUpdatedOn time.Time
	AOCUpdatedOn                 time.Time
}
