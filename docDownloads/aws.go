package docdownload

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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

// processAndUploadDocs preprocesses and uploads all tender documents to S3
func (d *DocDownloader) processAndUploadDocs(tenderID string, bucket string) error {
	baseDir := "TenderDocs/" + tenderID

	// Common helper: preprocess + upload
	uploadWithFolder := func(localFile, folder, prefix string) {
		files, err := PreprocessFile(localFile, baseDir)
		if err != nil {
			d.logger.Printf("[%s][docUpload] preprocess failed for %s: %v", d.state, localFile, err)
			return
		}
		for _, f := range files {
			flatName := FlattenPath(filepath.Base(f))
			key := fmt.Sprintf("tender-documents/%s/%s/%s%s", tenderID, folder, prefix, flatName)
			if err := uploadFileToS3(bucket, key, f); err != nil {
				d.logger.Printf("[%s][docUpload] upload failed: %v", d.state, err)
			} else {
				d.logger.Printf("[%s][docUpload] uploaded %s", d.state, key)
			}
		}
	}

	// --- NIT Docs ---
	for _, doc := range d.NITDocs {
		localFile := filepath.Join(baseDir, doc.DocumentName)
		uploadWithFolder(localFile, "nit-documents", "")
	}

	// --- WorkItem ZIP (or rar) ---
	if d.WorkItemZip.URL != "" {
		localFile := filepath.Join(baseDir, d.WorkItemZip.DocumentName)
		uploadWithFolder(localFile, "work-item-documents", "")
		// optional cleanup
		os.Remove(localFile)
	}

	// --- Corrigendum Docs ---
	for _, doc := range d.CorrigendumDocs {
		localFile := filepath.Join(baseDir, doc.DocumentName)
		// Corrigendum gets type prefix before filename
		uploadWithFolder(localFile, "latest-corrigendum-list", doc.Type+"_")
	}

	// --- Delete local folder ---
	if err := os.RemoveAll(baseDir); err != nil {
		d.logger.Printf("[%s][docUpload] cleanup failed: %v", d.state, err)
	} else {
		d.logger.Printf("[%s][docUpload] cleaned up %s", d.state, baseDir)
	}

	return nil
}
