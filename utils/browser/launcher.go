package browser

import (
	"os"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func NewBrowser() *rod.Browser {
	env := os.Getenv("APP_ENV") // "dev" or "prod"
	if env == "" {
		env = "production"
	}

	isHeadless := true
	if env == "dev" {
		isHeadless = false
	}

	// fmt.Printf("[browser] launching in %s mode (headless=%v)\n", env, isHeadless)

	u := launcher.New().
		Bin("/usr/bin/chromium"). // ensure correct path on Arch
		Headless(isHeadless).
		NoSandbox(true).
		Leakless(false). // must remain off for Arch
		MustLaunch()

	browser := rod.New().ControlURL(u).MustConnect()
	return browser
}
