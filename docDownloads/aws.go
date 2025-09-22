package docdownload

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// uploadFileToS3 uploads a local file to S3 under a specific key
func uploadFileToS3(bucket, key, filePath string) error {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return fmt.Errorf("unable to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg)

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("unable to open file %s: %w", filePath, err)
	}
	defer file.Close()

	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("failed to upload %s to S3: %w", filePath, err)
	}

	return nil
}

// processAndUploadDocs flattens, converts DOCX->PDF, and uploads to S3
func (d *DocDownloader) processAndUploadDocs(tenderID string, bucket string) error {
	baseDir := "TenderDocs" // folder where files are downloaded

	// Process NITDocs
	for _, doc := range d.NITDocs {
		localFile := filepath.Join(baseDir, doc.DocumentName) // prepend folder

		// Convert DOCX to PDF if needed
		if strings.HasSuffix(strings.ToLower(localFile), ".docx") {
			pdfFile, err := ConvertDocxToPDF(localFile)
			if err != nil {
				d.logger.Printf("[%s][docUpload] conversion failed: %v", d.state, err)
				continue
			}
			localFile = pdfFile
		}

		folder := "nit-documents"
		flatName := FlattenPath(filepath.Base(localFile)) // flatten only the filename
		key := fmt.Sprintf("tender-documents/%s/%s/%s", tenderID, folder, flatName)

		// Upload to S3
		if err := uploadFileToS3(bucket, key, localFile); err != nil {
			d.logger.Printf("[%s][docUpload] upload failed: %v", d.state, err)
		} else {
			d.logger.Printf("[%s][docUpload] successfully uploaded: %s", d.state, key)
		}
	}

	// Process Zip files
	if d.WorkItemZip.URL != "" {
		baseZip := filepath.Join(baseDir, d.WorkItemZip.DocumentName)
		tempExtractDir := filepath.Join(baseDir, "workitem_extracted")

		// Unzip
		if err := Unzip(baseZip, tempExtractDir); err != nil {
			d.logger.Printf("[%s][docUpload] failed to unzip WorkItem zip: %v", d.state, err)
		} else {
			d.logger.Printf("[%s][docUpload] unzipped WorkItem zip to %s", d.state, tempExtractDir)

			// Walk through all extracted files
			err := filepath.Walk(tempExtractDir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}

				localFile := path

				// Convert DOCX to PDF if needed
				if strings.HasSuffix(strings.ToLower(localFile), ".docx") {
					pdfFile, err := ConvertDocxToPDF(localFile)
					if err != nil {
						d.logger.Printf("[%s][docUpload] conversion failed: %v", d.state, err)
						return nil
					}
					localFile = pdfFile
				}

				flatName := FlattenPath(localFile[len(tempExtractDir)+1:]) // relative path flatten
				key := fmt.Sprintf("tender-documents/%s/work-item-documents/%s", tenderID, flatName)

				if err := uploadFileToS3(bucket, key, localFile); err != nil {
					d.logger.Printf("[%s][docUpload] upload failed: %v", d.state, err)
				} else {
					d.logger.Printf("[%s][docUpload] successfully uploaded: %s", d.state, key)
				}
				return nil
			})
			if err != nil {
				d.logger.Printf("[%s][docUpload] walking extracted files failed: %v", d.state, err)
			}
		}

		// Optional: delete the original zip
		os.Remove(baseZip)
		// Delete extracted folder
		os.RemoveAll(tempExtractDir)
	}

	// --- Delete local folder after upload ---
	if err := os.RemoveAll(baseDir); err != nil {
		d.logger.Printf("[%s][docUpload] failed to delete local folder %s: %v", d.state, baseDir, err)
	} else {
		d.logger.Printf("[%s][docUpload] local folder %s deleted successfully", d.state, baseDir)
	}

	return nil
}
