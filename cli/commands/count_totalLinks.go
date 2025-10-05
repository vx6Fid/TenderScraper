package commands

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/vx6fid/tender-scraper/utils"
)

func CountTotalLinks() error {
	runDate := utils.GetRunDate(false) // adjust path to your utils
	root := "/home/achal/Documents/Projects/TenderScraper/TenderData/Links/" + runDate
	fmt.Println("Scanning:", root)

	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}

	totalLinks := 0
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Folder\tRows")

	for _, entry := range entries {
		if entry.IsDir() {
			csvPath := filepath.Join(root, entry.Name(), "FinalLinks.csv")
			count, err := countRows(csvPath)
			if err != nil {
				fmt.Fprintf(w, "%s\t%s\n", entry.Name(), "--")
			} else {
				fmt.Fprintf(w, "%s\t%d\n", entry.Name(), count)
				totalLinks += count
			}
		}
	}

	fmt.Fprintln(w, "--------\t-------")
	fmt.Fprintf(w, "TOTAL\t%d\n", totalLinks)
	w.Flush()

	return nil
}

func countRows(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	rows := 0
	for {
		_, err := reader.Read()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return 0, err
		}
		rows++
	}
	return rows, nil
}
