package nav

import (
	"github.com/gocolly/colly/v2"
	"github.com/vx6fid/tender-scraper/scraper/captcha"
)

func ScrapeCorrigendum(c *colly.Collector, baseUrl string) {
	captcha.ManualCaptchaSolver("scraper/captcha.png")
}
