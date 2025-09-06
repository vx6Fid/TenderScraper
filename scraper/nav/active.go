package nav

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/vx6fid/tender-scraper/scraper/captcha"
)

// Global flag to avoid duplicate navigation
var visitedActive bool

// ScrapeActiveTenders - Main function that navigates to Active Tenders page
func ScrapeActiveTenders(c *colly.Collector, baseURL string) {
	fmt.Printf("[%s] Starting Active Tenders navigation\n", baseURL)

	// Handle navigation to Active Tenders page
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
			// Remove this handler so it doesn't fire again
			c.OnHTMLDetach("a")
		}
	})

	// Handle the Active Tenders form
	c.OnHTML("form#LatestActiveTenders", func(e *colly.HTMLElement) {
		fmt.Println("Reached Active Tenders form. Processing form...")
		ProcessActiveTendersForm(e)
	})

	// Handle tender list results - this should catch the results after form submission
	c.OnHTML("tr.odd, tr.even", func(e *colly.HTMLElement) {
		fmt.Println("Found tender row, processing...")
		cols := e.DOM.Find("td")

		if cols.Length() >= 7 { // Ensure we have enough columns
			// published := strings.TrimSpace(cols.Eq(1).Text())
			closing := strings.TrimSpace(cols.Eq(2).Text())
			// opening   := strings.TrimSpace(cols.Eq(3).Text())
			title := strings.TrimSpace(cols.Eq(4).Find("a").Text())
			link := e.Request.AbsoluteURL(cols.Eq(4).Find("a").AttrOr("href", ""))
			org := strings.TrimSpace(cols.Eq(5).Text())
			value := strings.TrimSpace(cols.Eq(6).Text())

			fmt.Printf("Title: %s\nLink: %s\nClosing: %s\nOrg: %s\nValue: %s\n\n",
				title, link, closing, org, value)
		}
	})

	// Save HTML response for debugging
	c.OnResponse(func(r *colly.Response) {
		fmt.Printf("Visited: %s (Status: %d)\n", r.Request.URL, r.StatusCode)

		// Save HTML to file for debugging if it contains tender results
		if strings.Contains(string(r.Body), "Active Tenders") ||
			strings.Contains(string(r.Body), "tr class=\"odd\"") ||
			strings.Contains(string(r.Body), "tr class=\"even\"") {

			filename := fmt.Sprintf("debug_response_%d.html", time.Now().Unix())
			if err := os.WriteFile(filename, r.Body, 0644); err != nil {
				fmt.Printf("Error saving debug file: %v\n", err)
			} else {
				fmt.Printf("Debug HTML saved to: %s\n", filename)
			}
		}
	})

	// Handle any errors
	c.OnError(func(r *colly.Response, err error) {
		fmt.Printf("Error visiting %s: %v (Status: %d)\n", r.Request.URL, err, r.StatusCode)
		// Save error response for debugging
		if r.Body != nil {
			filename := fmt.Sprintf("debug_error_%d.html", time.Now().Unix())
			if writeErr := os.WriteFile(filename, r.Body, 0644); writeErr != nil {
				fmt.Printf("Error saving error debug file: %v\n", writeErr)
			} else {
				fmt.Printf("Error response saved to: %s\n", filename)
			}
		}
	})

	if err := c.Visit(baseURL); err != nil {
		fmt.Printf("Error visiting base URL: %v\n", err)
	}
}

// ProcessActiveTendersForm - Handle captcha + form submission
func ProcessActiveTendersForm(e *colly.HTMLElement) {
	fmt.Println("=== PROCESSING ACTIVE TENDERS FORM ===")
	fmt.Printf("Form URL: %s\n", e.Request.URL)

	// Try to find captcha image - check multiple possible locations
	var captchaImg string

	// First try: direct child of form
	captchaImg = e.DOM.Find("img#captchaImage").AttrOr("src", "")
	if captchaImg == "" {
		// Second try: anywhere within the form
		captchaImg = e.DOM.Find("img[id*='captcha'], img[src*='captcha']").AttrOr("src", "")
	}
	if captchaImg == "" {
		// Third try: parent container
		captchaImg = e.DOM.Parent().Find("img#captchaImage, img[id*='captcha']").AttrOr("src", "")
	}
	if captchaImg == "" {
		// Fourth try: siblings or nearby elements
		captchaImg = e.DOM.Parent().Parent().Find("img[id*='captcha'], img[src*='captcha']").AttrOr("src", "")
	}

	if captchaImg == "" {
		fmt.Println("ERROR: Could not find captcha image in form")
		debugFormImages(e)
		return
	}

	fmt.Printf("Captcha image found: %s (len=%d)\n", captchaImg, len(captchaImg))

	// Convert relative URL to absolute if needed
	if strings.HasPrefix(captchaImg, "/") {
		captchaImg = e.Request.AbsoluteURL(captchaImg)
	}

	// Solve captcha
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
			fmt.Printf("Form field: %s = %s (type: %s)\n", name, value, inputType)
		}
	})

	// Handle select elements too
	e.ForEach("select", func(_ int, sel *colly.HTMLElement) {
		name := sel.Attr("name")
		if name != "" {
			// Get selected option value
			selectedValue := sel.DOM.Find("option[selected]").AttrOr("value", "")
			if selectedValue == "" {
				// Get first option value if none selected
				selectedValue = sel.DOM.Find("option").First().AttrOr("value", "")
			}
			formData[name] = selectedValue
			fmt.Printf("Select field: %s = %s\n", name, selectedValue)
		}
	})

	// Set required fields
	formData["captchaText"] = captchaSolution

	// Set empty values for search (to get all tenders)
	if _, exists := formData["TenderId"]; !exists {
		formData["TenderId"] = ""
	}
	if _, exists := formData["TenderTitle"]; !exists {
		formData["TenderTitle"] = ""
	}

	// Set submit button value
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

	// Add a small delay before submission
	time.Sleep(1 * time.Second)

	// Do POST submission
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
	fmt.Printf("Form method: %s\n", e.Attr("method"))

	// Check form content
	fmt.Println("Form HTML snippet:")
	html, _ := e.DOM.Html()
	fmt.Printf("%s\n", truncateString(html, 500))

	fmt.Println("All images in form:")
	e.ForEach("img", func(i int, img *colly.HTMLElement) {
		src := img.Attr("src")
		id := img.Attr("id")
		class := img.Attr("class")
		fmt.Printf("  [%d] src='%s' id='%s' class='%s' alt='%s'\n",
			i, truncateString(src, 80), id, class, img.Attr("alt"))
	})

	fmt.Println("All images in parent container:")
	e.DOM.Parent().Find("img").Each(func(i int, sel *goquery.Selection) {
		src, _ := sel.Attr("src")
		id, _ := sel.Attr("id")
		class, _ := sel.Attr("class")
		alt, _ := sel.Attr("alt")
		fmt.Printf("  [parent-%d] src='%s' id='%s' class='%s' alt='%s'\n",
			i, truncateString(src, 80), id, class, alt)
	})

	fmt.Println("All input fields:")
	e.ForEach("input", func(i int, input *colly.HTMLElement) {
		name := input.Attr("name")
		value := input.Attr("value")
		inputType := input.Attr("type")
		fmt.Printf("  [%d] name='%s' value='%s' type='%s'\n", i, name, value, inputType)
	})
}

// truncateString - Helper function to truncate strings for display
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
