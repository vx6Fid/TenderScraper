package docdownload

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/vx6fid/tender-scraper/utils"
)

func uploadFileToS3(bucket, key, filePath string) error {
	client, err := utils.NewS3Client(context.TODO())
	if err != nil {
		return err
	}

	if client == nil {
		return fmt.Errorf("S3 client is nil (AWS config likely invalid)")
	}

	// --- Skip if file already exists ---
	exists := false
	_, headErr := client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if headErr == nil {
		exists = true
	} else {
		var nsk *s3types.NotFound
		if !errors.As(headErr, &nsk) && !strings.Contains(headErr.Error(), "NotFound") {
			return fmt.Errorf("failed checking S3 object %s: %w", key, headErr)
		}
	}

	if exists {
		// Object already in S3 — skip quietly
		return nil
	}
	// --- Upload file if not found ---
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

func (d *DocDownloader) processAndUploadDocs(ctx context.Context, tenderID string, bucket string) error {
	baseDir := "TenderDocs/" + tenderID

	// Ensure baseDir exists
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create baseDir: %w", err)
	}

	// Helper: preprocess + upload a file to a folder in S3
	uploadWithFolder := func(localFile, folder, prefix string) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if _, err := os.Stat(localFile); os.IsNotExist(err) {
			// missing file — if it's the large-file case it was skipped earlier; treat as non-fatal but log
			d.logger.Printf("[%s][docUpload] missing (skipped or not downloaded): %s", d.state, localFile)
			return nil
		} else if err != nil {
			return fmt.Errorf("stat failed for %s: %w", localFile, err)
		}

		files, err := PreprocessFile(localFile, baseDir)
		if err != nil {
			return fmt.Errorf("[%s][docUpload] preprocess failed for %s: %v", d.state, localFile, err)
		}

		// per-upload retry
		const attempts = 3
		const baseDelay = 500 * time.Millisecond

		for _, f := range files {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			flatName := FlattenPath(filepath.Base(f))
			key := fmt.Sprintf("tender-documents/%s/%s/%s%s", tenderID, folder, prefix, flatName)
			uploadFn := func() error {
				return uploadFileToS3(bucket, key, f)
			}

			if err := Do(attempts, baseDelay, uploadFn); err != nil {
				// return the first hard upload error
				return fmt.Errorf("upload failed for %s -> %s: %w", f, key, err)
			}
		}

		return nil
	}

	// collect errors
	var errs []string

	// --- Collect files for ZIP (NIT + WorkItem) ---
	filesToZip := []string{}
	for _, doc := range d.NITDocs {
		localFile := filepath.Join(baseDir, doc.DocumentName)
		if _, err := os.Stat(localFile); err == nil {
			filesToZip = append(filesToZip, doc.DocumentName)
		}
	}
	if d.WorkItemZip.URL != "" {
		localFile := filepath.Join(baseDir, d.WorkItemZip.DocumentName)
		if _, err := os.Stat(localFile); err == nil {
			filesToZip = append(filesToZip, d.WorkItemZip.DocumentName)
		}
	}

	// --- Create zip if needed and upload
	if len(filesToZip) > 0 && !d.skipWorkNit {
		zipPath := filepath.Join(baseDir, "TenderDocs.zip")
		if err := createZip(zipPath, baseDir, filesToZip); err != nil {
			errs = append(errs, fmt.Sprintf("zip creation failed: %v", err))
		} else {
			key := fmt.Sprintf("tender-documents/%s/TenderDocs.zip", tenderID)
			if err := Do(3, 500*time.Millisecond, func() error {
				return uploadFileToS3(bucket, key, zipPath)
			}); err != nil {
				errs = append(errs, fmt.Sprintf("zip upload failed: %v", err))
			}
		}
	}

	// Upload NIT docs
	for _, doc := range d.NITDocs {
		localFile := filepath.Join(baseDir, doc.DocumentName)
		if err := uploadWithFolder(localFile, "nit-documents", ""); err != nil {
			errs = append(errs, err.Error())
		}
	}

	// Upload WorkItem ZIP
	if d.WorkItemZip.URL != "" {
		localFile := filepath.Join(baseDir, d.WorkItemZip.DocumentName)
		if err := uploadWithFolder(localFile, "work-item-documents", ""); err != nil {
			errs = append(errs, err.Error())
		}
	}

	// Upload Corrigendum docs
	for _, doc := range d.CorrigendumDocs {
		localFile := filepath.Join(baseDir, doc.DocumentName)
		if err := uploadWithFolder(localFile, "latest-corrigendum-list", doc.Type+"_"); err != nil {
			errs = append(errs, err.Error())
		}
	}

	// --- Cleanup ---
	if err := os.RemoveAll(baseDir); err != nil {
		// not fatal, so only logging
		d.logger.Printf("[%s][docUpload] cleanup failed: %v", d.state, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("processAndUploadDocs errors: %s", strings.Join(errs, "; "))
	}

	return nil
}
