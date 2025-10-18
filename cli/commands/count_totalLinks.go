// Modify this file to include the number of links in search.csv and corrigendum.csv along with FinalLinks.csv
// Subtract the header row from count, also it is not counting FinalCSV if either search or corrigendum is empty
package commands

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/vx6fid/tender-scraper/utils"
)

func CountTotalLinks() error {
	runDate := utils.GetRunDate(false) // adjust path to your utils
	var dataDir string
	flag.StringVar(&dataDir, "data-dir", "TenderData", "Path to TenderData directory")
	flag.Parse()
	root := filepath.Join(dataDir, "Links", runDate)
	fmt.Println("Scanning:", root)

	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}

	totalLinks := 0
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Folder\tSearch\tCorrigendum\tFinal")

	for _, entry := range entries {
		if entry.IsDir() {

			FinalCSVPath := filepath.Join(root, entry.Name(), "FinalLinks.csv")
			SearchCSVPath := filepath.Join(root, entry.Name(), "search.csv")
			CorrigendumCSVPath := filepath.Join(root, entry.Name(), "corrigendums.csv")

			countFinal, err1 := countRows(FinalCSVPath)
			countSearch, err2 := countRows(SearchCSVPath)
			countCorrigendum, err3 := countRows(CorrigendumCSVPath)
			if err1 != nil {
				countFinal = 0
			}
			if err2 != nil {
				countSearch = 0
			}
			if err3 != nil {
				countCorrigendum = 0
			}

			fmt.Fprintf(w, "%s\t%d\t%d\t%d\n", entry.Name(), max(0, countSearch-1), max(0, countCorrigendum-1), max(0, countFinal-1))
			totalLinks += max(countFinal, countSearch)
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
			fmt.Println("Error reading CSV:", err)
			if err.Error() == "EOF" {
				break
			}
			return 0, err
		}
		rows++
	}
	return rows, nil
}
