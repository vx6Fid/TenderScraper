package pastTenders

import (
	"fmt"

	"github.com/gocolly/colly/v2"
	"github.com/vx6fid/tender-scraper/session"
)

type PastTender struct {
	collector   *colly.Collector
	url         string
	currentPage int

	// csvWriter *csv.Writer
}

func NewPastTender(sess *session.Session, url string, domain string) *PastTender {
	collector := sess.NewCollector(domain)
	return &PastTender{
		collector:   collector,
		url:         url,
		currentPage: 1,
		// csvWriter:   csvWriter,
	}
}

func (pt *PastTender) Extract(tenderData *TenderData, pastTenderData *PastTendersData) error {
	var viewMoreHandled, viewSummaryHandled bool

	pt.collector.OnHTML(`a[title="View More Details"]`, func(e *colly.HTMLElement) {
		if viewMoreHandled {
			return
		}
		viewMoreHandled = true
		href := e.Attr("href")
		absHref := e.Request.AbsoluteURL(href)
		fmt.Printf("(View More Details) link: %s\n", absHref)
		pt.setupTenderDataHandler(pt.collector, tenderData, absHref)
	})

	pt.collector.OnHTML(`a[title="View the all stage summary Details"]`, func(e *colly.HTMLElement) {
		if viewSummaryHandled {
			return
		}
		viewSummaryHandled = true
		href := e.Attr("href")
		absHref := e.Request.AbsoluteURL(href)
		fmt.Printf("(View the all stage summary Details) link: %s\n", absHref)
		pt.setupPastTenderDataHandler(pt.collector, pastTenderData, absHref)
	})

	return pt.collector.Visit(pt.url)
}
