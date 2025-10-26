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

// uploadFileToS3 uploads a local file to S3 under a specific key
// func uploadFileToS3(bucket, key, filePath string) error {
// 	cfg, err := config.LoadDefaultConfig(context.TODO())
// 	if err != nil {
// 		return fmt.Errorf("unable to load AWS config: %w", err)
// 	}

// 	client := s3.NewFromConfig(cfg)

// 	file, err := os.Open(filePath)
// 	if err != nil {
// 		return fmt.Errorf("unable to open file %s: %w", filePath, err)
// 	}
// 	defer file.Close()

// 	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
// 		Bucket: aws.String(bucket),
// 		Key:    aws.String(key),
// 		Body:   file,
// 	})
// 	if err != nil {
// 		return fmt.Errorf("failed to upload %s to S3: %w", filePath, err)
// 	}

// 	return nil
// }

func uploadFileToS3(bucket, key, filePath string) error {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return fmt.Errorf("unable to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg)

	// --- Skip if file already exists ---
	_, err = client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		// Object exists, skip upload
		return fmt.Errorf("[S3] Skipping existing file: %s\n", key)
	}
	var nsk *s3types.NotFound
	if !errors.As(err, &nsk) && !strings.Contains(err.Error(), "NotFound") {
		// HeadObject failed for another reason
		return fmt.Errorf("failed checking S3 object: %w", err)
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

// processAndUploadDocs preprocesses and uploads all tender documents to S3
// func (d *DocDownloader) processAndUploadDocs(tenderID string, bucket string) error {
// 	baseDir := "TenderDocs/" + tenderID

// 	// Common helper: preprocess + upload
// 	uploadWithFolder := func(localFile, folder, prefix string) {
// 		if _, err := os.Stat(localFile); os.IsNotExist(err) {
// 			d.logger.Printf("[%s][docUpload] skipping missing file (probably >1GB): %s", d.state, localFile)
// 			return
// 		}

// 		files, err := PreprocessFile(localFile, baseDir)
// 		if err != nil {
// 			d.logger.Printf("[%s][docUpload] preprocess failed for %s: %v", d.state, localFile, err)
// 			return
// 		}
// 		for _, f := range files {
// 			flatName := FlattenPath(filepath.Base(f))
// 			key := fmt.Sprintf("tender-documents/%s/%s/%s%s", tenderID, folder, prefix, flatName)
// 			if err := uploadFileToS3(bucket, key, f); err != nil {
// 				d.logger.Printf("[%s][docUpload]: %v", d.state, err)
// 			} else {
// 				d.logger.Printf("[%s][docUpload] uploaded %s", d.state, key)
// 			}
// 		}
// 	}

// 	// --- NIT Docs ---
// 	for _, doc := range d.NITDocs {
// 		localFile := filepath.Join(baseDir, doc.DocumentName)
// 		uploadWithFolder(localFile, "nit-documents", "")
// 	}

// 	// --- WorkItem ZIP (or rar) ---
// 	if d.WorkItemZip.URL != "" {
// 		localFile := filepath.Join(baseDir, d.WorkItemZip.DocumentName)
// 		uploadWithFolder(localFile, "work-item-documents", "")
// 		// optional cleanup
// 		os.Remove(localFile)
// 	}

// 	// --- Corrigendum Docs ---
// 	for _, doc := range d.CorrigendumDocs {
// 		localFile := filepath.Join(baseDir, doc.DocumentName)
// 		// Corrigendum gets type prefix before filename
// 		uploadWithFolder(localFile, "latest-corrigendum-list", doc.Type+"_")
// 	}

// 	// Ensure baseDir exists
// 	if err := os.MkdirAll(baseDir, 0755); err != nil {
// 		return fmt.Errorf("failed to create baseDir: %w", err)
// 	}

// 	// Collect relative paths for NIT + WorkItem docs
// 	filesToZip := []string{}
// 	for _, doc := range d.NITDocs {
// 		filesToZip = append(filesToZip, doc.DocumentName)
// 	}
// 	if d.WorkItemZip.URL != "" {
// 		filesToZip = append(filesToZip, d.WorkItemZip.DocumentName)
// 	}

// 	zipPath := filepath.Join(baseDir, "TenderDocs.zip")

// 	// Only attempt ZIP if files exist
// 	if len(filesToZip) > 0 {
// 		args := append([]string{"-r", zipPath}, filesToZip...)
// 		cmd := exec.Command("zip", args...)
// 		cmd.Dir = baseDir

// 		if output, err := cmd.CombinedOutput(); err != nil {
// 			d.logger.Printf("[%s][docUpload] zip creation failed: %v, output: %s", d.state, err, string(output))
// 		} else {
// 			key := fmt.Sprintf("tender-documents/%s/TenderDocs.zip", tenderID)
// 			if err := uploadFileToS3(bucket, key, zipPath); err != nil {
// 				d.logger.Printf("[%s][docUpload] zip upload failed: %v", d.state, err)
// 			} else {
// 				d.logger.Printf("[%s][docUpload] uploaded TenderDocs.zip to S3", d.state)
// 			}
// 		}
// 	}

// 	// --- Delete local folder ---
// 	if err := os.RemoveAll(baseDir); err != nil {
// 		d.logger.Printf("[%s][docUpload] cleanup failed: %v", d.state, err)
// 	} else {
// 		d.logger.Printf("[%s][docUpload] cleaned up %s", d.state, baseDir)
// 	}

// 	return nil
// }

func (d *DocDownloader) processAndUploadDocs(tenderID string, bucket string) error {
	baseDir := "TenderDocs/" + tenderID

	// Ensure baseDir exists
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create baseDir: %w", err)
	}

	// Helper: preprocess + upload a file to a folder in S3
	uploadWithFolder := func(localFile, folder, prefix string) {
		if _, err := os.Stat(localFile); os.IsNotExist(err) {
			d.logger.Printf("[%s][docUpload] skipping missing file (probably >1GB): %s", d.state, localFile)
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
				d.logger.Printf("[%s][docUpload] uploaded %s", d.state, key)
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
		d.logger.Printf("[%s][docUpload][DEBUG] skipping TenderDocs ZIP", d.state)
	}

	// --- Create ZIP of NIT + WorkItem docs ---
	if len(filesToZip) > 0 && !d.skipWorkNit {
		zipPath := "TenderDocs.zip"

		// Debug: print baseDir, zipPath
		d.logger.Printf("[%s][docUpload][DEBUG] baseDir = %s", d.state, baseDir)
		d.logger.Printf("[%s][docUpload][DEBUG] zipPath = %s", d.state, zipPath)

		// Debug: print each file to be zipped and check existence
		for _, f := range filesToZip {
			fpath := filepath.Join(baseDir, f)
			if info, err := os.Stat(fpath); err != nil {
				d.logger.Printf("[%s][docUpload][DEBUG] file missing: %s (err=%v)", d.state, fpath, err)
			} else {
				d.logger.Printf("[%s][docUpload][DEBUG] file exists: %s (size=%d)", d.state, fpath, info.Size())
			}
		}

		// Debug: print working directory
		if cwd, err := os.Getwd(); err == nil {
			d.logger.Printf("[%s][docUpload][DEBUG] current working dir = %s", d.state, cwd)
		} else {
			d.logger.Printf("[%s][docUpload][DEBUG] could not get current working dir: %v", d.state, err)
		}

		// Print the command to be executed
		args := append([]string{"-r", zipPath}, filesToZip...)
		d.logger.Printf("[%s][docUpload][DEBUG] zip command: zip %v", d.state, args)

		cmd := exec.Command("zip", args...)
		cmd.Dir = baseDir

		if output, err := cmd.CombinedOutput(); err != nil {
			d.logger.Printf("[%s][docUpload] zip creation failed: %v, output: %s", d.state, err, string(output))
		} else {
			key := fmt.Sprintf("tender-documents/%s/TenderDocs.zip", tenderID)
			if err := uploadFileToS3(bucket, key, filepath.Join(baseDir, "TenderDocs.zip")); err != nil {
				d.logger.Printf("[%s][docUpload] zip upload failed: %v", d.state, err)
			} else {
				d.logger.Printf("[%s][docUpload] uploaded TenderDocs.zip to S3", d.state)
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
		d.logger.Printf("[%s][docUpload] cleanup failed: %v", d.state, err)
	} else {
		d.logger.Printf("[%s][docUpload] cleaned up %s", d.state, baseDir)
	}

	return nil
}
