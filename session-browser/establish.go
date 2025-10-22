package session_browser

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/vx6fid/tender-scraper/scraper/captcha"
)

func EstablishSession(b *rod.Browser, baseURL, state string) (*rod.Page, error) {
	activeTendersURL := fmt.Sprintf("%s?page=FrontEndLatestActiveTenders&service=page", baseURL)

	page := b.MustPage(activeTendersURL)
	page.MustWaitLoad()

	// log.Printf("[%s] Page loaded: %s", state, page.MustInfo().URL)

	// 1. Detect if captcha exists
	hasCaptcha, _, _ := page.Has("form#LatestActiveTenders")
	if hasCaptcha {
		// log.Printf("[%s] Captcha form detected", state)
		if err := handleCaptchaWithRetry(page, state, 3); err != nil {
			return nil, fmt.Errorf("[%s] captcha handling failed: %w", state, err)
		}
	} else {
		log.Printf("[%s] No captcha form detected, proceeding directly", state)
		return page, nil
	}

	// 2. Wait for tender table
	page.MustWaitLoad()
	page.MustWaitElementsMoreThan("table#table tr", 1)

	// log.Printf("[%s] Tender table detected", state)
	return page, nil
}

func handleCaptchaWithRetry(page *rod.Page, state string, maxRetries int) error {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// log.Printf("[%s] Attempt %d to solve captcha", state, attempt)

		// Locate captcha image
		img, err := page.ElementX(`//img[
			contains(translate(@id, 'CAPTCHA', 'captcha'), 'captcha') or
			contains(translate(@src, 'CAPTCHA', 'captcha'), 'captcha') or
			contains(translate(@name, 'CAPTCHA', 'captcha'), 'captcha')
		]`)
		if err != nil {
			return fmt.Errorf("[%s] captcha image not found", state)
		}

		src, _ := img.Attribute("src")
		if src == nil {
			return fmt.Errorf("[%s] captcha image src missing", state)
		}

		code, err := captcha.LocalCaptchaSolver(*src, log.Default())
		if err != nil {
			return fmt.Errorf("[%s] captcha solver failed: %w", state, err)
		}

		// Fill and submit
		page.MustElement(`input[name="captchaText"]`).MustInput(strings.TrimSpace(code))
		page.MustElement(`input[type="submit"]`).MustClick()
		time.Sleep(500 * time.Millisecond) // let DOM start updating

		var success, failure bool
		var errorMsg string

		for range 40 { // up to ~20s
			// check for table rows > 1
			val, err := page.Eval(`() => {
						const t = document.getElementById("table");
						return t ? t.rows.length : 0;
					}`)
			if err == nil && val.Value.Int() > 1 {
				success = true
				break
			}

			// check for error text
			if ok, _, _ := page.Has(`.error`); ok {
				if errEl, e := page.Element(`.error`); e == nil {
					txt, _ := errEl.Text()
					if strings.Contains(strings.ToLower(txt), "invalid captcha") {
						errorMsg = strings.TrimSpace(txt)
						failure = true
						break
					}
				}
			}
			time.Sleep(500 * time.Millisecond)
		}

		if success {
			// log.Printf("[%s] Captcha solved successfully on attempt %d", state, attempt)
			return nil
		}

		if failure {
			log.Printf("[%s] Captcha attempt %d failed: %s", state, attempt, errorMsg)
			continue
		}

		log.Printf("[%s] Captcha attempt %d inconclusive — retrying", state, attempt)
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("[%s] failed to solve captcha after %d attempts", state, maxRetries)
}
