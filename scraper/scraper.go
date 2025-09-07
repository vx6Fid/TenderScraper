package scraper

import (
	"log"
	"net/http/cookiejar"
	"time"

	"github.com/gocolly/colly/v2"
)

func NewCollector(allowedDomains ...string) *colly.Collector {
	c := colly.NewCollector(
		colly.AllowedDomains(allowedDomains...),
		colly.MaxDepth(3),
	)

	// Create cookie jar for session management
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Printf("Error creating cookie jar: %v", err)
	}
	c.SetCookieJar(jar)

	// Set browser headers
	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36")
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.9")
		r.Headers.Set("Accept-Encoding", "gzip, deflate, br")
		r.Headers.Set("DNT", "1")
		r.Headers.Set("Connection", "keep-alive")
		r.Headers.Set("Upgrade-Insecure-Requests", "1")

		if r.Method == "POST" {
			r.Headers.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Headers.Set("Cache-Control", "no-cache")
			r.Headers.Set("Origin", "https://"+r.URL.Host)
			r.Headers.Set("Referer", "https://"+r.URL.Host+"/nicgep/app?page=FrontEndLatestActiveTenders&service=page")
		}

		log.Printf("Request: %s %s", r.Method, r.URL.String())
	})

	c.OnResponse(func(r *colly.Response) {
		log.Printf("Response: %s | Status: %d | Size: %d bytes",
			r.Request.URL.String(), r.StatusCode, len(r.Body))
	})

	c.OnError(func(r *colly.Response, err error) {
		log.Printf("Error: %s | %v", r.Request.URL.String(), err)
	})

	// Set reasonable limits
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       2 * time.Second,
	})

	c.SetRequestTimeout(30 * time.Second)

	return c
}
