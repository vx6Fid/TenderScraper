package nav

import (
	"fmt"
	"strings"

	"github.com/gocolly/colly/v2"
	"github.com/vx6fid/tender-scraper/scraper/captcha"
)

// Global flag to avoid duplicate navigation
var visitedActive bool

// ScrapeActiveTenders - Main function that navigates to Active Tenders page
func ScrapeActiveTenders(c *colly.Collector, baseURL string) {
	fmt.Printf("[%s] Starting Active Tenders navigation\n", baseURL)

	c.OnHTML("a", func(e *colly.HTMLElement) {
		if visitedActive {
			return // skip redundant click
		}
		linkText := strings.TrimSpace(e.Attr("title"))
		if linkText == "" {
			linkText = strings.TrimSpace(e.Text)
		}
		if strings.EqualFold(linkText, "Active Tenders") ||
			strings.Contains(strings.ToLower(linkText), "active tender") {
			visitedActive = true
			link := e.Request.AbsoluteURL(e.Attr("href"))
			fmt.Printf("Visiting Active Tenders link: %s\n", link)
			if err := e.Request.Visit(link); err != nil {
				fmt.Printf("Error visiting Active Tenders page: %v\n", err)
			}

			// removing this handle so it doesn't fire again
			c.OnHTMLDetach("a")
		}
	})

	c.OnHTML("form#LatestActiveTenders", func(e *colly.HTMLElement) {
		fmt.Println("Reached Active Tenders form. Processing form...")
		ProcessActiveTendersForm(e)
	})

	c.OnHTML("tr.odd, tr.even", func(e *colly.HTMLElement) {
		cols := e.DOM.Find("td")

		// published := strings.TrimSpace(cols.Eq(1).Text())
		closing := strings.TrimSpace(cols.Eq(2).Text())
		// opening   := strings.TrimSpace(cols.Eq(3).Text())
		title := strings.TrimSpace(cols.Eq(4).Find("a").Text())
		link := e.Request.AbsoluteURL(cols.Eq(4).Find("a").AttrOr("href", ""))
		org := strings.TrimSpace(cols.Eq(5).Text())
		value := strings.TrimSpace(cols.Eq(6).Text())

		fmt.Printf("Title: %s\nLink: %s\nClosing: %s\nOrg: %s\nValue: %s\n\n",
			title, link, closing, org, value)
	})

	c.OnResponse(func(r *colly.Response) {
		fmt.Printf("Visited: %s\n", r.Request.URL)
	})

	if err := c.Visit(baseURL); err != nil {
		fmt.Printf("Error visiting base URL: %v\n", err)
	}
}

// ProcessActiveTendersForm - Handle captcha + form submission
func ProcessActiveTendersForm(e *colly.HTMLElement) {
	fmt.Println("=== PROCESSING ACTIVE TENDERS FORM ===")
	fmt.Printf("Form URL: %s\n", e.Request.URL)

	// Captcha is present in View Source -> should be detectable here
	captchaImg := e.DOM.Parent().Find("img#captchaImage").AttrOr("src", "")
	if captchaImg == "" {
		fmt.Println("ERROR: Could not find captcha image in form")
		debugFormImages(e)
		return
	}

	fmt.Printf("Captcha image found (len=%d)\n", len(captchaImg))

	// Solve captcha (pass base64 string or absolute URL)
	captchaSolution, err := captcha.ManualCaptchaSolver(captchaImg)
	if err != nil {
		fmt.Printf("Error solving captcha: %v\n", err)
		return
	}
	fmt.Printf("Captcha solved: %s\n", captchaSolution)

	// Submit form with solution
	submitActiveTendersForm(e, captchaSolution)
}

func submitActiveTendersForm(e *colly.HTMLElement, captchaSolution string) {
	formData := make(map[string]string)

	// Extract all input fields (hidden + text)
	e.ForEach("input", func(_ int, input *colly.HTMLElement) {
		name := input.Attr("name")
		value := input.Attr("value")
		inputType := strings.ToLower(input.Attr("type"))
		if name != "" && inputType != "submit" {
			formData[name] = value
		}
	})

	// Explicitly set required fields
	formData["captchaText"] = captchaSolution
	formData["TenderId"] = ""    // empty = all tenders
	formData["TenderTitle"] = "" // empty = all tenders
	formData["Submit"] = "Search"

	// Build absolute action URL
	actionURL := e.Attr("action")
	if actionURL == "" {
		actionURL = e.Request.URL.String()
	} else {
		actionURL = e.Request.AbsoluteURL(actionURL)
	}

	fmt.Printf("Submitting to: %s\n", actionURL)
	fmt.Printf("Form data: %+v\n", formData)

	// Do POST
	if err := e.Request.Post(actionURL, formData); err != nil {
		fmt.Printf("Error submitting form: %v\n", err)
	} else {
		fmt.Println("Form submitted successfully!")
	}
}

// debugFormImages - Helper function to debug image detection issues
func debugFormImages(e *colly.HTMLElement) {
	fmt.Println("=== FORM DEBUG INFO ===")
	fmt.Printf("Form ID: %s\n", e.Attr("id"))
	fmt.Printf("Form action: %s\n", e.Attr("action"))

	fmt.Println("All images in form:")
	e.ForEach("img", func(i int, img *colly.HTMLElement) {
		src := img.Attr("src")
		fmt.Printf("  [%d] src='%s' id='%s' name='%s' alt='%s'\n",
			i, truncateString(src, 80), img.Attr("id"), img.Attr("name"), img.Attr("alt"))
	})
}

// truncateString - Helper function to truncate strings for display
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
