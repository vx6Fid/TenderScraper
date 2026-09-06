package session

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/vx6fid/tender-scraper/scraper/captcha"
)

type Session struct {
	Jar                   http.CookieJar
	BaseURL               string
	ActiveTendersURL      string
	CorrigendumURL        string
	ResultsURL            string
	captchaSolved         bool
	sessionEstablished    bool
	docSessionEstablished bool
	logger                *log.Logger

	// internal collector used only for captcha/session establishment
	captchaCollector *colly.Collector
}

func NewSession(baseURL string, state string) *Session {
	jar, _ := cookiejar.New(nil)

	logDir := "TenderData/Logs/sessions"
	logFileName := fmt.Sprintf("%s/%s_%s.txt", logDir, state, time.Now().Format("02_Jan_2006_15_04_05"))

	var logger *log.Logger
	if err := os.MkdirAll(logDir, 0755); err != nil {
		// Don't kill the process over a log file — fall back to stderr.
		log.Printf("failed to create session log dir, logging to stderr: %v", err)
		logger = log.New(os.Stderr, "", log.LstdFlags)
	} else if logFile, err := os.Create(logFileName); err != nil {
		log.Printf("failed to create session log file, logging to stderr: %v", err)
		logger = log.New(os.Stderr, "", log.LstdFlags)
	} else {
		logger = log.New(logFile, "", log.LstdFlags)
	}

	return &Session{
		Jar:              jar,
		BaseURL:          baseURL,
		logger:           logger,
		ActiveTendersURL: baseURL + "?page=FrontEndLatestActiveTenders&service=page",
		CorrigendumURL:   baseURL + "?page=FrontEndLatestActiveCorrigendums&service=page",
		ResultsURL:       baseURL + "?page=WebTenderStatusLists&service=page",
	}
}

// NewCollector returns a new colly.Collector that shares this session's cookie jar.
// Pass the allowed domain(s) (usually the hostname of BaseURL).
func (s *Session) NewCollector(allowedDomains ...string) *colly.Collector {
	opts := []colly.CollectorOption{}
	if len(allowedDomains) > 0 {
		opts = append(opts, colly.AllowedDomains(allowedDomains...))
	}
	c := colly.NewCollector(opts...)

	// attach the session cookie jar
	c.SetCookieJar(s.Jar)

	// reasonable defaults / headers
	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36")
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.9")
		if r.Method == "POST" {
			r.Headers.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Headers.Set("Cache-Control", "no-cache")
			r.Headers.Set("Origin", "https://"+r.URL.Host)
			r.Headers.Set("Referer", "https://"+r.URL.Host+"/nicgep/app?page=FrontEndLatestActiveTenders&service=page")
		}
		// s.logger.Printf("[request] %s %s", r.Method, r.URL.String())
	})

	// c.OnResponse(func(r *colly.Response) {
	// 	s.logger.Printf("[response] %s | %d | %d bytes", r.Request.URL.String(), r.StatusCode, len(r.Body))
	// })

	c.OnError(func(r *colly.Response, err error) {
		if r != nil && r.Request != nil {
			s.logger.Printf("[error] %s | %d | %v", r.Request.URL.String(), r.StatusCode, err)
		} else {
			s.logger.Printf("[error] %v", err)
		}
	})

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       400 * time.Millisecond,
	})

	c.SetRequestTimeout(40 * time.Second)
	return c
}

// EstablishSession performs the captcha flow using an internal collector
// and populates the session's cookie jar. It waits (with timeout) for the
// captcha flow to confirm the session.
func (s *Session) EstablishSession(ctx context.Context, sessionType string) error {
	// build captcha collector bound to host
	host := HostFromURL(s.BaseURL)
	s.captchaCollector = s.NewCollector(host)

	// bind handlers
	switch sessionType {
	case "ActiveTenders":
		s.captchaCollector.OnHTML("form#LatestActiveTenders", func(e *colly.HTMLElement) {
			s.handleCaptchaForm(e)
		})
	case "CorrigendumTenders":
		s.captchaCollector.OnHTML("form#LatestActiveCorrigendums", func(e *colly.HTMLElement) {
			s.handleCaptchaForm(e)
		})
		// case "TenderStatus":
		// 	s.captchaCollector.OnHTML("form#frmSearchFilter", func(e *colly.HTMLElement) {
		// 		s.handleCaptchaForm(e)
		// 	})
	}

	s.captchaCollector.OnResponse(func(r *colly.Response) {
		s.handleCaptchaResponse(r)
	})
	s.captchaCollector.OnError(func(r *colly.Response, err error) {
		s.handleError(r, err)
	})

	switch sessionType {
	case "ActiveTenders":
		// s.logger.Printf("STEP 1: Starting captcha/session flow: %s", s.ActiveTendersURL)
		if err := s.captchaCollector.Visit(s.ActiveTendersURL); err != nil {
			return fmt.Errorf("failed to visit active tenders page for captcha: %w", err)
		}
	case "CorrigendumTenders":
		// s.logger.Printf("STEP 1: Starting captcha/session flow: %s", s.CorrigendumURL)
		if err := s.captchaCollector.Visit(s.CorrigendumURL); err != nil {
			return fmt.Errorf("failed to visit corrigendum tenders page for captcha: %w", err)
		}
		// case "TenderStatus":
		// 	s.logger.Printf("STEP 1: Starting captcha/session flow: %s", s.ResultsURL)
		// 	if err := s.captchaCollector.Visit(s.ResultsURL); err != nil {
		// 		return fmt.Errorf("failed to visit tender status page for captcha: %w", err)
		// 	}
	}

	// wait until either sessionEstablished is true or timeout
	timeout := time.After(30 * time.Second)
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for captcha/session establishment")
		case <-tick.C:
			if s.sessionEstablished {
				u, _ := url.Parse(s.BaseURL)
				cookies := s.Jar.Cookies(u)
				if len(cookies) == 0 {
					return fmt.Errorf("no cookies found after captcha; session not established")
				}
				s.logger.Printf("Session established: %d cookies", len(cookies))
				return nil
			}
		}
	}
}

// ValidateSession returns true if sessionEstablished and cookies exist for BaseURL.
func (s *Session) ValidateSession() bool {
	if !s.sessionEstablished {
		return false
	}
	u, err := url.Parse(s.BaseURL)
	if err != nil {
		return false
	}
	cookies := s.Jar.Cookies(u)
	return len(cookies) > 0
}

func (s *Session) SessionEstablished() bool    { return s.sessionEstablished }
func (s *Session) DocSessionEstablished() bool { return s.docSessionEstablished }
func (s *Session) MarkDocSessionEstablished()  { s.docSessionEstablished = true }

/* ----- internal handlers ----- */

// solveCaptcha routes to a captcha solver based on the CAPTCHA_MODE env var:
//   - "manual": prompt on stdin and fill by hand (for local testing)
//   - anything else / unset: the local OCR solver (default, unchanged behavior)
func solveCaptcha(captchaSrc string, logger *log.Logger) (string, error) {
	if strings.EqualFold(os.Getenv("CAPTCHA_MODE"), "manual") {
		return captcha.ManualStdinCaptchaSolver(captchaSrc, logger)
	}
	return captcha.LocalCaptchaSolver(captchaSrc, logger)
}

func (s *Session) handleCaptchaForm(e *colly.HTMLElement) {
	// if s.captchaSolved {
	// 	s.logger.Println("[captcha] already solved, skipping")
	// 	return
	// }
	// s.logger.Printf("[captcha] found form (status %d) at %s", e.Response.StatusCode, e.Request.URL.String())

	// Skip captcha if the main tender table exists
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(e.Response.Body)))
	if err != nil {
		// s.logger.Printf("[captcha] parse error: %v", err)
		return
	}

	s.logger.Printf("[captcha] find table in document")
	if doc.Find("table#table").Length() > 0 {
		// s.logger.Println("[captcha] tender list table found, skipping captcha")
		s.sessionEstablished = true // mark session ready
		return
	}

	// Otherwise, solve captcha
	// s.logger.Printf("[captcha] found form (status %d) at %s", e.Response.StatusCode, e.Request.URL.String())
	s.captchaSolved = true

	// find captcha image (try multiple selectors)
	captchaSrc := e.DOM.Find("img#captchaImage").AttrOr("src", "")
	if captchaSrc == "" {
		captchaSrc = e.DOM.Find("img[id*='captcha'], img[src*='captcha']").AttrOr("src", "")
	}
	if captchaSrc == "" {
		captchaSrc = e.DOM.Parent().Find("img#captchaImage, img[id*='captcha']").AttrOr("src", "")
	}
	if captchaSrc == "" {
		// s.logger.Println("[captcha] no captcha image found")
		return
	}

	// s.logger.Printf("[captcha] image found!")

	// pick solver based on CAPTCHA_MODE (defaults to the local OCR solver)
	sol, err := solveCaptcha(captchaSrc, s.logger)
	if err != nil {
		s.logger.Printf("[captcha] solver error: %v", err)
		s.captchaSolved = false
		return
	}

	// s.logger.Printf("[captcha] solved: %s", sol)
	s.submitCaptchaForm(e, sol)
}

func (s *Session) submitCaptchaForm(e *colly.HTMLElement, captchaSolution string) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(e.Response.Body)))
	if err != nil {
		s.logger.Printf("[captcha] parse error: %v", err)
		return
	}

	formData := map[string]string{}
	// First try form#LatestActiveTenders inputs
	doc.Find("form#LatestActiveTenders").Each(func(_ int, f *goquery.Selection) {
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
	formData["Submit"] = "Search"

	// s.logger.Printf("[captcha] submitting form with %d fields", len(formData))
	if err := e.Request.Post(s.BaseURL, formData); err != nil {
		s.logger.Printf("[captcha] post failed: %v", err)
	}
}

func (s *Session) handleCaptchaResponse(r *colly.Response) {
	// s.logger.Printf("[captcha] response handler: %s | %s | %d", r.Request.Method, r.Request.URL.String(), r.StatusCode)
	if s.captchaSolved || r.Request.Method != "POST" {
		return
	}
	body := string(r.Body)

	hasError := strings.Contains(strings.ToLower(body), "invalid input request") ||
		strings.Contains(strings.ToLower(body), "incorrect") ||
		strings.Contains(strings.ToLower(body), "wrong")

	stillHasCaptcha := strings.Contains(body, "captchaImage") || strings.Contains(body, "captchaText")

	if hasError {
		s.logger.Printf("[captcha] server rejected captcha (status %d)", r.StatusCode)
		return
	}
	if stillHasCaptcha {
		// s.logger.Printf("[captcha] server still showing captcha after submit (status %d)", r.StatusCode)
		return
	}

	// assume success
	s.captchaSolved = true

	// check cookies in jar for BaseURL
	u, _ := url.Parse(s.BaseURL)
	if cookies := s.Jar.Cookies(u); len(cookies) > 0 {
		// s.logger.Printf("[captcha] session cookies received: %d", len(cookies))
		for _, c := range cookies {
			s.logger.Printf("  cookie: %s = %s", c.Name, c.Value)
		}
	} else {
		// s.logger.Printf("[captcha] no cookies found even though captcha looked successful")
	}
	s.sessionEstablished = true
}

func (s *Session) handleError(r *colly.Response, err error) {
	if r == nil || r.Request == nil {
		s.logger.Printf("[session error] %v", err)
		return
	}
	s.logger.Printf("[session error] url=%s status=%d err=%v", r.Request.URL.String(), r.StatusCode, err)
}

/* helpers */
func HostFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
