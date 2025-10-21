package active

import (
	"log"

	"github.com/go-rod/rod"
)

func Run(state string, totalRows int, page *rod.Page) error {
	err := ExtractTenders(state, totalRows, page)
	if err != nil {
		return err
	}

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

	log.Printf("Extracted %d tenders from page", serial-1)
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
