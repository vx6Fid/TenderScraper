// scraper.go provides common utilities and base configuration
// for all scrapers (e.g., Colly collector setup, shared data types).

package scraper

import (
	"log"
	"net/http/cookiejar"

	"github.com/gocolly/colly/v2"
)

// NewCollector creates a base Colly collector with default settings
func NewCollector(allowedDomains ...string) *colly.Collector {
	c := colly.NewCollector(
		colly.AllowedDomains(allowedDomains...),
		colly.MaxDepth(3),
	)

	// Create a real cookie jar
	jar, _ := cookiejar.New(nil)
	c.SetCookieJar(jar)

	c.OnRequest(func(r *colly.Request) {
		log.Printf("Visiting %s with cookies: %v", r.URL.String(), c.Cookies(r.URL.String()))
	})

	c.OnError(func(r *colly.Response, err error) {
		log.Printf("Error: %s -> %v\n", r.Request.URL.String(), err)
	})

	return c
}
