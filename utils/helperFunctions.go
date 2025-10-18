package utils

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/vx6fid/tender-scraper/session"
)

// GetBaseURLAndState finds the baseURL and state for a given tenderURL
func GetBaseURLAndState(tenderURL string) (string, string, error) {
	parsed, err := url.Parse(tenderURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid tenderURL: %w", err)
	}

	host := parsed.Hostname()

	for _, entry := range BaseURLs {
		if strings.EqualFold(host, entry.Domain) {
			return entry.BaseURL, entry.State, nil
		}
	}

	return "", "", fmt.Errorf("no matching baseURL found for host %s", host)
}

func FetchTotalPages(sess *session.Session, baseURL string, domain string) (int, error) {
	collector := sess.NewCollector(domain)
	var totalPages int
	var scrapeErr error

	collector.OnHTML("a#linkLast", func(e *colly.HTMLElement) {
		href := e.Attr("href")
		// fmt.Println("[TotalPages] ", href)
		// href looks like: "...&sp=AFrontEndAdvancedSearchResult%2Ctable&sp=399"
		u, err := url.Parse(href)
		if err != nil {
			scrapeErr = fmt.Errorf("parse href: %w", err)
			return
		}
		q := u.Query()
		spParams := q["sp"]
		if len(spParams) == 0 {
			scrapeErr = fmt.Errorf("no sp param found in href=%s", href)
			return
		}

		// Get the last sp parameter (which should contain the page number)
		lastSpParam := spParams[len(spParams)-1]

		// Parse the page number from the last sp parameter
		p, err := strconv.Atoi(lastSpParam)
		if err != nil {
			scrapeErr = fmt.Errorf("invalid page number %q: %w", lastSpParam, err)
			return
		}
		totalPages = p
	})

	searchPage1 := BuildPageURLRaw(baseURL, 1)
	err := collector.Visit(searchPage1)
	if err != nil {
		return 0, fmt.Errorf("visit failed: %w", err)
	}

	if scrapeErr != nil {
		return 0, scrapeErr
	}
	if totalPages == 0 {
		totalPages = 1
	}
	return totalPages, nil
}

func BuildPageURLRaw(baseURL string, currentPage int) string {
	return fmt.Sprintf("%s?component=$TablePages.linkPage&page=FrontEndAdvancedSearchResult&service=direct&session=T&sp=AFrontEndAdvancedSearchResult%%2Ctable&sp=%d", baseURL, currentPage)
}

func GiveStageName() string {
	fmt.Printf("Choose one of the Tender Types:\n")
	for key, value := range StageName {
		fmt.Printf("%s: %s\n", key, value)
	}
	fmt.Println("Enter the tender type:")
	var tenderType string
	fmt.Scanln(&tenderType)
	return tenderType
}

// GetRunDate returns the last created folder in Links or PastLinks
func GetRunDate(pastTenders bool) string {
	dirPath := "TenderData/Links"
	if pastTenders {
		dirPath = "TenderData/PastLinks"
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		log.Printf("Error reading directory: %v", err)
		return ""
	}

	var dates []time.Time
	nameMap := make(map[time.Time]string)

	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			parsed, err := time.Parse("02_Jan_2006", name)
			if err != nil {
				log.Printf("Skipping invalid folder name: %s", name)
				continue
			}
			dates = append(dates, parsed)
			nameMap[parsed] = name
		}
	}

	if len(dates) == 0 {
		return ""
	}

	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	latest := dates[len(dates)-1]

	return nameMap[latest]
}

// Extract last [...] as Tender ID from search.csv TenderID field
func ExtractTenderID(s string) string {
	re := regexp.MustCompile(`\[(2[^\]]+)\]`)
	matches := re.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(matches[len(matches)-1][1])
}

var spaceRe = regexp.MustCompile(`[\s\p{Zs}]+`)

func CleanField(s string) string {
	s = strings.TrimSpace(s)
	s = spaceRe.ReplaceAllString(s, " ")
	return s
}
