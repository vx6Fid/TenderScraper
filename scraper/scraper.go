// scraper.go - Fixed version with proper session management
package scraper

import (
	"log"
	"net/http/cookiejar"
	"time"

	"github.com/gocolly/colly/v2"
)

// NewCollector creates a base Colly collector with proper session management
func NewCollector(allowedDomains ...string) *colly.Collector {
	c := colly.NewCollector(
		colly.AllowedDomains(allowedDomains...),
		colly.MaxDepth(5), // Increased depth for form submissions
	)

	// Create cookie jar for session management
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Printf("Error creating cookie jar: %v", err)
	}
	c.SetCookieJar(jar)

	// Set proper headers to mimic a real browser
	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.9")
		r.Headers.Set("Accept-Encoding", "gzip, deflate")
		r.Headers.Set("Connection", "keep-alive")
		r.Headers.Set("Upgrade-Insecure-Requests", "1")

		// For POST requests, set proper content type
		if r.Method == "POST" {
			r.Headers.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Headers.Set("Cache-Control", "no-cache")
			r.Headers.Set("Referer", r.URL.String())
		}

		log.Printf("Request: %s %s", r.Method, r.URL.String())
		cookies := c.Cookies(r.URL.String())
		if len(cookies) > 0 {
			log.Printf("Cookies: %d cookies sent", len(cookies))
		}
	})

	c.OnResponse(func(r *colly.Response) {
		log.Printf("Response: %s -> %d bytes (Status: %d)",
			r.Request.URL.String(), len(r.Body), r.StatusCode)

		// Log received cookies
		if cookie := r.Headers.Get("Set-Cookie"); cookie != "" {
			log.Printf("New cookies received: %s", cookie)
		}
	})

	c.OnError(func(r *colly.Response, err error) {
		log.Printf("Error: %s -> %v (Status: %d)",
			r.Request.URL.String(), err, r.StatusCode)
	})

	// Set delays to avoid being blocked
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       2 * time.Second,
	})

	return c
}
