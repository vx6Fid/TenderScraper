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

func Unrar(src, dest string) error {
	if err := os.MkdirAll(dest, os.ModePerm); err != nil {
		return err
	}
	cmd := exec.Command("unrar", "x", "-o+", src, dest)
	return cmd.Run()
}

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

// PreprocessFile takes an input file, expands/converts if needed, and returns a list of final files ready to upload.
func PreprocessFile(inputPath, workDir string) ([]string, error) {
	ext := strings.ToLower(filepath.Ext(inputPath))

	switch ext {
	case ".zip":
		// Extract zip
		extractDir := filepath.Join(workDir, strings.TrimSuffix(filepath.Base(inputPath), ext))
		if err := Unzip(inputPath, extractDir); err != nil {
			return nil, fmt.Errorf("failed to unzip %s: %w", inputPath, err)
		}
		return collectAndProcessFiles(extractDir, workDir)

	case ".rar":
		// Extract rar
		extractDir := filepath.Join(workDir, strings.TrimSuffix(filepath.Base(inputPath), ext))
		if err := Unrar(inputPath, extractDir); err != nil {
			return nil, fmt.Errorf("failed to unrar %s: %w", inputPath, err)
		}
		return collectAndProcessFiles(extractDir, workDir)

	case ".docx":
		// Convert DOCX → PDF
		pdfFile, err := ConvertDocxToPDF(inputPath)
		if err != nil {
			return nil, err
		}
		return []string{pdfFile}, nil

	default:
		// Already a final file
		return []string{inputPath}, nil
	}
}

func collectAndProcessFiles(dir, workDir string) ([]string, error) {
	var results []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		files, err := PreprocessFile(path, workDir)
		if err != nil {
			return err
		}
		results = append(results, files...)
		return nil
	})
	return results, err
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

	// Download corrigendum documents
	for i, doc := range d.CorrigendumDocs {
		filePath := filepath.Join(baseDir, doc.DocumentName)
		d.logger.Printf("[%s][docDownload] downloading corrigendum doc %d/%d: %s",
			d.state, i+1, len(d.CorrigendumDocs), filePath)

		if err := DownloadFile(doc.URL, filePath, d.sess.Jar); err != nil {
			d.logger.Printf("[%s][docDownload] corrigendum doc download failed: %v", d.state, err)
		} else {
			d.logger.Printf("[%s][docDownload] successfully downloaded: %s", d.state, filePath)
		}
	}

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
	d.CorrigendumDocs = nil
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

type CorrigendumDocs struct {
	DocumentName   string
	DocumentSizeKB string
	URL            string
}

type CorrigendumDocument struct {
	DocumentName string
	Type         string
	URL          string
}
