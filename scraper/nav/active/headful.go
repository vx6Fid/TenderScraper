package active

import (
	"fmt"
	"log"
	"time"

	"github.com/go-rod/rod"
	session_browser "github.com/vx6fid/tender-scraper/session-browser"
	"github.com/vx6fid/tender-scraper/utils/types"
)

func Run(b *rod.Browser, u types.URLS) error {
	page, _ := session_browser.EstablishSession(b, u.BaseURL, u.State)
	// if err != nil {
	// 	return fmt.Errorf("[%s] session establishment failed: %w", u.State, err)
	// }
	// log.Printf("[%s] Session established", u.State)

	page = b.MustPage(u.BaseURL + "?component=%24DirectLink&page=FrontEndTendersByOrganisation&service=direct&session=T")
	page.MustWaitLoad()

	// Poll table for population
	var prevCount, currCount int
	for range 60 {
		val, err := page.Eval(`() => document.getElementById("table").rows.length`)
		if err != nil {
			return fmt.Errorf("[%s] counting rows failed: %w", u.BaseURL, err)
		}
		currCount = int(val.Value.Int())
		if currCount == 0 {
			time.Sleep(1 * time.Second)
			continue
		}
		if currCount == prevCount {
			break
		}
		prevCount = currCount
		time.Sleep(1 * time.Second)
	}

	if currCount == 0 {
		return fmt.Errorf("[%s] table did not populate any rows", u.State)
	}
	// log.Printf("[%s] Rows: %d", u.State, currCount)

	if err := ExtractTenders(u.State, currCount, page); err != nil {
		return fmt.Errorf("[%s] active links extraction failed: %w", u.State, err)
	}

	// Close extra tabs
	// activePages, _ := b.Pages()
	// for _, p := range activePages {
	// 	if p.MustInfo().URL != "about:blank" {
	// 		if err := p.Close(); err != nil {
	// 			log.Printf("failed to close tab: %v", err)
	// 		}
	// 	}
	// }
	return nil
}

func ExtractTenders(state string, totalRows int, page *rod.Page) error {
	const batchSize = 100
	serial := 1
	// base, _ := url.Parse(page.MustInfo().URL)

	writeHeader := true
	for i := 1; i < totalRows; i += batchSize { // skip header
		end := i + batchSize
		end = min(end, totalRows)

		// evaluate JS to extract batch of rows as plain data
		batchData := page.MustEval(`
		(start, end) => {
			const trs = Array.from(document.getElementById("table").rows).slice(start, end);
		    return trs.map(tr => {
		        const tds = Array.from(tr.cells);
		        let link = "";
		        const a = tds[4]?.querySelector('a');
		        if (a) link = a.href;
		        return {
		            serial: null, // will assign in Go
		            title: tds[4]?.innerText || "",
		            organisation: tds[5]?.innerText || "",
		            publishedDate: tds[1]?.innerText || "",
		            closingDate: tds[2]?.innerText || "",
		            link: link,
		            uniqueIdentifier: tds[4]?.innerText || ""
		        }
		    });
		}`, i, end)

		batch := []Tender{}
		for _, row := range batchData.Arr() {
			obj := row.Map()
			tender := Tender{
				Serial:           serial,
				Title:            obj["title"].Str(),
				Organisation:     obj["organisation"].Str(),
				PublishedDate:    obj["publishedDate"].Str(),
				ClosingDate:      obj["closingDate"].Str(),
				Link:             obj["link"].Str(),
				UniqueIdentifier: obj["uniqueIdentifier"].Str(),
			}
			if IsInvalidTender(tender) {
				continue
			}
			batch = append(batch, tender)
			serial++
		}
		if len(batch) > 0 {
			SaveTendersCSVBatch(state, batch, writeHeader)
			writeHeader = false
		}

	}

	log.Printf("[%s] Extracted %d tenders from page", state, serial-1)
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
