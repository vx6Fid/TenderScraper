package docdownload

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func uploadFileToS3(bucket, key, filePath string) error {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return fmt.Errorf("unable to load AWS config: %w", err)
	}

	if cfg.Region == "" {
		return fmt.Errorf("AWS region missing in configuration")
	}

	client := s3.NewFromConfig(cfg)

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

func (d *DocDownloader) processAndUploadDocs(tenderID string, bucket string) error {
	baseDir := "TenderDocs/" + tenderID

	// Ensure baseDir exists
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create baseDir: %w", err)
	}

	// Helper: preprocess + upload a file to a folder in S3
	uploadWithFolder := func(localFile, folder, prefix string) {
		if _, err := os.Stat(localFile); os.IsNotExist(err) {
			// d.logger.Printf("[%s][docUpload] skipping missing file (probably >1GB): %s", d.state, localFile)
			return
		}

		files, err := PreprocessFile(localFile, baseDir)
		if err != nil {
			d.logger.Printf("[%s][docUpload] preprocess failed for %s: %v", d.state, localFile, err)
			return
		}
		for _, f := range files {
			flatName := FlattenPath(filepath.Base(f))
			key := fmt.Sprintf("tender-documents/%s/%s/%s%s", tenderID, folder, prefix, flatName)
			if err := uploadFileToS3(bucket, key, f); err != nil {
				d.logger.Printf("[%s][docUpload]: %v", d.state, err)
			} else {
				// d.logger.Printf("[%s][docUpload] uploaded %s", d.state, key)
			}
		}
	}

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

	if d.skipWorkNit {
		// d.logger.Printf("[%s][docUpload][DEBUG] skipping TenderDocs ZIP", d.state)
	}

	// --- Create ZIP of NIT + WorkItem docs ---
	if len(filesToZip) > 0 && !d.skipWorkNit {
		zipPath := "TenderDocs.zip"

		// Debug: print working directory
		if _, err := os.Getwd(); err == nil {
			// d.logger.Printf("[%s][docUpload][DEBUG] current working dir = %s", d.state, cwd)
		} else {
			d.logger.Printf("[%s][docUpload][DEBUG] could not get current working dir: %v", d.state, err)
		}

		// Print the command to be executed
		args := append([]string{"-r", zipPath}, filesToZip...)

		cmd := exec.Command("zip", args...)
		cmd.Dir = baseDir

		if output, err := cmd.CombinedOutput(); err != nil {
			d.logger.Printf("[%s][docUpload] zip creation failed: %v, output: %s", d.state, err, string(output))
		} else {
			key := fmt.Sprintf("tender-documents/%s/TenderDocs.zip", tenderID)
			if err := uploadFileToS3(bucket, key, filepath.Join(baseDir, "TenderDocs.zip")); err != nil {
				d.logger.Printf("[%s][docUpload] zip upload failed: %v", d.state, err)
			}
		}
	}

	// --- Upload individual NIT docs ---
	for _, doc := range d.NITDocs {
		localFile := filepath.Join(baseDir, doc.DocumentName)
		uploadWithFolder(localFile, "nit-documents", "")
	}

	// --- Upload WorkItem ZIP ---
	if d.WorkItemZip.URL != "" {
		localFile := filepath.Join(baseDir, d.WorkItemZip.DocumentName)
		uploadWithFolder(localFile, "work-item-documents", "")
	}

	// --- Upload Corrigendum docs ---
	for _, doc := range d.CorrigendumDocs {
		localFile := filepath.Join(baseDir, doc.DocumentName)
		uploadWithFolder(localFile, "latest-corrigendum-list", doc.Type+"_")
	}

	// --- Cleanup ---
	if err := os.RemoveAll(baseDir); err != nil {
		// d.logger.Printf("[%s][docUpload] cleanup failed: %v", d.state, err)
	} else {
		// d.logger.Printf("[%s][docUpload] cleaned up %s", d.state, baseDir)
	}

	return nil
}
