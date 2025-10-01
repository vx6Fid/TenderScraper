package utils

import (
	"fmt"
	"time"
)

// GetDateRange returns the start and end dates for the past tender data
func GetDateRange() (string, string) {
	// read from and to date
	fmt.Println("Enter start date(DD/MM/YYYY):")
	var from string
	fmt.Scanln(&from)

	fmt.Println("Enter end date(DD/MM/YYYY):")
	var to string
	fmt.Scanln(&to)

	return from, to
}

// SplitDateRange splits [from, to] into chunks of size daysPerGroup.
func SplitDateRange(from, to time.Time, daysPerGroup int) [][2]time.Time {
	var ranges [][2]time.Time
	cur := from
	for cur.Before(to) {
		end := cur.AddDate(0, 0, daysPerGroup-1) // inclusive end
		if end.After(to) {
			end = to
		}
		ranges = append(ranges, [2]time.Time{cur, end})
		cur = end.AddDate(0, 0, 1) // next chunk starts after end
	}
	return ranges
}

// FormatDate converts time.Time to the dd/mm/yyyy format required by the scraper.
func FormatDate(t time.Time) string {
	return t.Format("02/01/2006")
}
