package docdownload

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/vx6fid/tender-scraper/scraper/captcha"
	"github.com/vx6fid/tender-scraper/session"
)

// HandleDocDownloadCaptchaForm handles the captcha form on DocDownload page.
func (d *DocDownloader) HandleDocDownloadCaptchaForm(e *colly.HTMLElement) {
	if d.sess.DocSessionEstablished() {
		d.logger.Println("[docDownload][captcha] already solved, skipping")
		return
	}

	d.logger.Printf("[docDownload][captcha] found form (status %d) at %s", e.Response.StatusCode, e.Request.URL.String())

	// This is a dedicated captcha page, so we always need to solve it
	// No content checks needed since this page only shows captcha

	// Locate captcha image (try multiple selectors like the working code)
	captchaSrc := e.DOM.Find("img#captchaImage").AttrOr("src", "")
	if captchaSrc == "" {
		captchaSrc = e.DOM.Find("img[id*='captcha'], img[src*='captcha']").AttrOr("src", "")
	}
	if captchaSrc == "" {
		captchaSrc = e.DOM.Parent().Find("img#captchaImage, img[id*='captcha']").AttrOr("src", "")
	}
	if captchaSrc == "" {
		d.logger.Println("[docDownload][captcha] no captcha image found")
		return
	}

	d.logger.Printf("[docDownload][captcha] image found!")

	// Solve captcha (blocks until solution ready)
	sol, err := captcha.LocalCaptchaSolver(captchaSrc, d.logger)
	if err != nil {
		d.logger.Printf("[docDownload][captcha] solver error: %v", err)
		return
	}

	d.logger.Printf("[docDownload][captcha] solution: %s", sol)
	d.submitDocDownloadCaptchaForm(e, sol)
}

func (d *DocDownloader) submitDocDownloadCaptchaForm(e *colly.HTMLElement, captchaSolution string) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(e.Response.Body)))
	if err != nil {
		d.logger.Printf("[docDownload][captcha] parse error: %v", err)
		return
	}

	formData := map[string]string{}

	// First try form#frmCaptcha inputs (based on the HTML structure)
	doc.Find("form#frmCaptcha").Each(func(_ int, f *goquery.Selection) {
		f.Find("input[name]").Each(func(_ int, in *goquery.Selection) {
			if name, ok := in.Attr("name"); ok && name != "" {
				val, _ := in.Attr("value")
				formData[name] = val
			}
		})
	})

	// Fallback: collect broad inputs (avoid known noise like domainUrl)
	if len(formData) < 5 {
		doc.Find("input[name]").Each(func(_ int, in *goquery.Selection) {
			if name, ok := in.Attr("name"); ok && name != "" && name != "domainUrl" {
				// try to ignore inputs from WebBorder_0 to avoid irrelevant fields
				if in.Closest("form#WebBorder_0").Length() == 0 {
					val, _ := in.Attr("value")
					formData[name] = val
				}
			}
		})
	}

	// Override captcha field and submit
	formData["captchaText"] = captchaSolution
	formData["Submit"] = "Submit"

	d.logger.Printf("[docDownload][captcha] submitting form with %d fields", len(formData))

	// Construct proper action URL - the form action is "/eprocure/app"
	actionURL := "https://" + session.HostFromURL(d.sess.BaseURL) + "/nicgep/app"
	fmt.Println("actionURL: ", actionURL)
	if err := e.Request.Post(actionURL, formData); err != nil {
		d.logger.Printf("[docDownload][captcha] post failed: %v", err)
	}
}

// HandleDocDownloadCaptchaResponse checks the response after captcha submit.
func (d *DocDownloader) HandleDocDownloadCaptchaResponse(r *colly.Response) {
	d.logger.Printf("[docDownload][captcha] response handler: %s | %s | %d", r.Request.Method, r.Request.URL.String(), r.StatusCode)

	// Only process POST responses
	if r.Request.Method != "POST" {
		return
	}

	body := string(r.Body)

	// Check for error messages
	hasError := strings.Contains(strings.ToLower(body), "invalid input request") ||
		strings.Contains(strings.ToLower(body), "incorrect") ||
		strings.Contains(strings.ToLower(body), "wrong")

	// Check if captcha still present
	stillHasCaptcha := strings.Contains(body, "captchaImage") || strings.Contains(body, "captchaText")

	if hasError {
		d.logger.Printf("[docDownload][captcha] server rejected captcha (status %d)", r.StatusCode)
		return
	}
	if stillHasCaptcha {
		d.logger.Printf("[docDownload][captcha] server still showing captcha after submit (status %d)", r.StatusCode)
		return
	}

	// assume success
	d.logger.Println("[docDownload][captcha] captcha accepted")

	// Update cookies (using the same pattern as working code)
	u, _ := url.Parse(d.sess.BaseURL)
	if cookies := d.sess.Jar.Cookies(u); len(cookies) > 0 {
		d.logger.Printf("[docDownload][captcha] session cookies received: %d", len(cookies))
		for _, c := range cookies {
			d.logger.Printf("  cookie: %s = %s", c.Name, c.Value)
		}
	} else {
		d.logger.Printf("[docDownload][captcha] no cookies found even though captcha looked successful")
	}

	d.sess.MarkDocSessionEstablished()
}

// parseCookieString parses a single Set-Cookie string
func parseCookieString(raw string) *http.Cookie {
	parts := strings.Split(raw, ";")
	kv := strings.SplitN(parts[0], "=", 2)
	if len(kv) != 2 {
		return nil
	}
	return &http.Cookie{
		Name:  strings.TrimSpace(kv[0]),
		Value: strings.TrimSpace(kv[1]),
	}
}
