package docdownload

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
)

const maxFileSize = 1 << 30 // 1 GB

// DownloadFile saves a file locally using the given cookies.
func DownloadFile(url, filePath string, jar http.CookieJar) error {
	client := &http.Client{Jar: jar}

	// --- HEAD request to check file size ---
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return fmt.Errorf("HEAD request creation failed: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HEAD request failed: %w", err)
	}
	resp.Body.Close() // no need to read body

	cl := resp.Header.Get("Content-Length")
	if cl != "" {
		size, err := strconv.ParseInt(cl, 10, 64)
		if err == nil && size > maxFileSize {
			return fmt.Errorf("skipping download, file too large (%d bytes)", size)
		}
	}

	// --- Proceed to download ---
	resp, err = client.Get(url)
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
