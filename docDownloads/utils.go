package docdownload

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Unzip extracts a zip archive to a specified folder
func Unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fPath := filepath.Join(dest, f.Name)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fPath, os.ModePerm); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fPath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// FlattenPath converts a path like "f1/f2/work.pdf" -> "f1_f2_work.pdf"
func FlattenPath(path string) string {
	cleanPath := filepath.Clean(path)
	parts := strings.Split(cleanPath, string(filepath.Separator))
	return strings.Join(parts, "_")
}

// ConvertDocxToPDF converts a DOCX file to PDF using LibreOffice
func ConvertDocxToPDF(docxPath string) (string, error) {
	pdfPath := strings.TrimSuffix(docxPath, filepath.Ext(docxPath)) + ".pdf"
	cmd := exec.Command("libreoffice", "--headless", "--convert-to", "pdf", docxPath, "--outdir", filepath.Dir(docxPath))
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to convert %s to PDF: %w", docxPath, err)
	}
	return pdfPath, nil
}

// downloadFiles downloads all collected NIT documents and zip files
func (d *DocDownloader) downloadFiles() error {
	if len(d.NITDocs) == 0 && d.WorkItemZip.URL == "" {
		d.logger.Printf("[%s][docDownload] no documents found to download", d.state)
		return nil
	}

	// Ensure TenderDocs folder exists
	baseDir := "TenderDocs"
	if err := os.MkdirAll(baseDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create folder %s: %w", baseDir, err)
	}

	// Download NIT documents
	for i, doc := range d.NITDocs {
		filePath := filepath.Join(baseDir, doc.DocumentName)
		d.logger.Printf("[%s][docDownload] downloading NIT doc %d/%d: %s",
			d.state, i+1, len(d.NITDocs), filePath)

		if err := DownloadFile(doc.URL, filePath, d.sess.Jar); err != nil {
			d.logger.Printf("[%s][docDownload] NIT doc download failed: %v", d.state, err)
		} else {
			d.logger.Printf("[%s][docDownload] successfully downloaded: %s", d.state, filePath)
		}
	}

	// Download zip files
	// Download zip file (if exists)
	if d.WorkItemZip.URL != "" {
		baseDir := "TenderDocs"
		filePath := filepath.Join(baseDir, d.WorkItemZip.DocumentName)
		d.logger.Printf("[%s][docDownload] downloading zip file: %s", d.state, filePath)
		if err := DownloadFile(d.WorkItemZip.URL, filePath, d.sess.Jar); err != nil {
			d.logger.Printf("[%s][docDownload] zip download failed: %v", d.state, err)
		} else {
			d.logger.Printf("[%s][docDownload] successfully downloaded: %s", d.state, filePath)
		}
	}

	return nil
}

// GetResults returns the extracted documents and links
func (d *DocDownloader) GetResults() ([]NITDocument, WorkItemDocument) {
	return d.NITDocs, d.WorkItemZip
}

// Reset clears the extracted documents and links for reuse
func (d *DocDownloader) Reset() {
	d.NITDocs = nil
	d.WorkItemZip = WorkItemDocument{}
	d.logger.Printf("[%s][docDownload] reset completed", d.state)
}

// Close cleans up the collector resources
func (d *DocDownloader) Close() {
	if d.collector != nil {
		d.collector = nil
		d.logger.Printf("[%s][docDownload] collector cleaned up", d.state)
	}
}

// DocumentLink holds any document link found on the tender page
type DocumentLink struct {
	URL  string
	Text string
	Type string // "docDownoad", "zip", etc.
}

// NITDocument holds the NIT doc metadata + download link
type NITDocument struct {
	SerialNo       string
	DocumentName   string
	Description    string
	DocumentSizeKB string
	URL            string
}

type WorkItemDocument struct {
	DocumentName   string
	DocumentSizeKB string
	URL            string
}
