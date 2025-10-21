package active

import (
	"log"
	"net/url"
	"strings"

	"github.com/go-rod/rod"
)

func Run(state string, page *rod.Page) error {
	tenders, err := ExtractTenders(page)
	if err != nil {
		return err
	}

	if err := SaveTendersCSV(state, tenders); err != nil {
		return err
	}

	return nil
}

func ExtractTenders(page *rod.Page) ([]Tender, error) {
	tenders := []Tender{}

	// Wait for the table to load
	page.MustWaitElementsMoreThan("table#table tr", 1)

	rows := page.MustElements("table#table tr")
	serial := 1
	base, _ := url.Parse(page.MustInfo().URL)

	for i, row := range rows {
		if i == 0 {
			continue // skip header row
		}
		tender, err := ExtractTenderInfo(row, serial, base)
		if err != nil {
			return nil, err
		}
		if IsInvalidTender(tender) {
			continue
		}
		tenders = append(tenders, tender)

		serial++
	}

	log.Printf("Extracted %d tenders from page", len(tenders))
	return tenders, nil
}

func ExtractTenderInfo(row *rod.Element, serial int, base *url.URL) (Tender, error) {
	tender := Tender{}
	cells := row.MustElements("td")
	if len(cells) < 6 {
		return Tender{}, nil // skip header/empty rows
	}

	title := ExtractTitle(cells[4].MustText())
	organisation := strings.TrimSpace(cells[5].MustText())
	publishedDate := strings.TrimSpace(cells[1].MustText())
	closingDate := strings.TrimSpace(cells[2].MustText())

	// get link
	linkElem, err := cells[4].Element("a") // returns nil + error if not found
	fullLink := ""
	if err == nil && linkElem != nil {
		href, _ := linkElem.Attribute("href")
		if href != nil {
			rel, _ := url.Parse(*href)
			fullLink = base.ResolveReference(rel).String()
		}
	}

	tender = Tender{
		Serial:           serial,
		Title:            title,
		Organisation:     organisation,
		PublishedDate:    publishedDate,
		ClosingDate:      closingDate,
		Link:             fullLink,
		UniqueIdentifier: cells[4].MustText(),
	}

	return tender, nil
}
