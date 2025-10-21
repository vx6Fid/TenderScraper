package session_browser

import (
	"fmt"
	"log"
	"strings"

	"github.com/go-rod/rod"
	"github.com/vx6fid/tender-scraper/scraper/captcha"
)

func EstablishSession(b *rod.Browser, baseURL, state string) (*rod.Page, error) {
	activeTendersURL := fmt.Sprintf("%s?page=FrontEndLatestActiveTenders&service=page", baseURL)

	page := b.MustPage(activeTendersURL)
	page.MustWaitLoad()

	log.Printf("[%s] Page loaded: %s", state, page.MustInfo().URL)

	// 1. Detect if captcha exists
	hasCaptcha, _, _ := page.Has("form#LatestActiveTenders")
	if hasCaptcha {
		log.Printf("[%s] Captcha form detected", state)
		if err := handleCaptcha(page, state); err != nil {
			return nil, fmt.Errorf("[%s] captcha handling failed: %w", state, err)
		}
	} else {
		log.Printf("[%s] No captcha form detected, proceeding directly", state)
		return page, nil
	}

	// 2. Wait for tender table
	page.MustWaitLoad()
	page.MustWaitElementsMoreThan("table#table tr", 1)

	log.Printf("[%s] Tender table detected", state)
	return page, nil
}

func handleCaptcha(page *rod.Page, state string) error {
	// Locate captcha image
	log.Printf("[%s] Searching for captcha image...", state)

	// Find *any* <img> whose id/src/name contains 'captcha' (case-insensitive)
	img, err := page.ElementX(`//img[
			contains(translate(@id, 'CAPTCHA', 'captcha'), 'captcha') or
			contains(translate(@src, 'CAPTCHA', 'captcha'), 'captcha') or
			contains(translate(@name, 'CAPTCHA', 'captcha'), 'captcha')
		]`)
	if err != nil {
		html, _ := page.HTML()
		log.Printf("[%s] No <img> with captcha substring found. Dumping first 500 chars of HTML:\n%s", state, html)
		return fmt.Errorf("[%s] captcha image not found", state)
	}

	src, _ := img.Attribute("src")
	if src == nil {
		log.Printf("[%s] Captcha <img> found but no src attribute", state)
		return fmt.Errorf("[%s] captcha image src missing", state)
	}

	log.Printf("[%s] Captcha image found: %s", state, *src)

	// Get Captcha Solution
	code, err := captcha.LocalCaptchaSolver(*src, log.Default())
	if err != nil {
		return fmt.Errorf("solver failed: %w", err)
	}

	log.Printf("[%s] Captcha solved: %s", state, code)

	// Fill and submit
	page.MustElement(`input[name="captchaText"]`).MustInput(strings.TrimSpace(code))

	// Click submit button
	submitBtn := page.MustElement(`input[type="submit"]`)
	submitBtn.MustClick()
	log.Printf("[%s] Submitted captcha form", state)

	page.MustWaitLoad()
	return nil
}
