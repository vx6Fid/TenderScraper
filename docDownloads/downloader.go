package docdownload

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

// DownloadFile saves a file locally using the given cookies.
func DownloadFile(url, filePath string, jar http.CookieJar) error {
	client := &http.Client{Jar: jar}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("file create failed: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
