package main

import (
	"log"
	"strings"
	"sync"

	"github.com/joho/godotenv"
	docdownload "github.com/vx6fid/tender-scraper/docDownloads"
	"github.com/vx6fid/tender-scraper/session"
	"github.com/vx6fid/tender-scraper/utils"
	types "github.com/vx6fid/tender-scraper/utils/types"
)

type TaskStatus string

const (
	StatusProcessing TaskStatus = "processing"
	StatusCompleted  TaskStatus = "completed"
	StatusFailed     TaskStatus = "failed"
)

type Task struct {
	TenderID string
	Status   TaskStatus
	Message  string
}

var (
	jobQueue = make(chan func(), 100) // max 100 queued jobs
)

func worker(id int) {
	for job := range jobQueue {
		log.Printf("Worker %d starting job", id)
		job()
		log.Printf("Worker %d finished job", id)
	}
}

func StartWorkerPool(n int) {
	for i := 0; i < n; i++ {
		go worker(i + 1)
	}
}

var (
	taskStore = make(map[string]*Task)
	mu        sync.RWMutex
)

func LoadEnvOrFatal() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
}

func runTenderDownload(tenderID, tenderURL string, corrigendumLinks []types.CorrLinks, baseURLs []types.URLS) {
	mu.Lock()
	taskStore[tenderID] = &Task{TenderID: tenderID, Status: StatusProcessing}
	mu.Unlock()

	defer func() {
		// Clean up or update final status on panic
		if r := recover(); r != nil {
			mu.Lock()
			taskStore[tenderID].Status = StatusFailed
			taskStore[tenderID].Message = "Panic occurred"
			mu.Unlock()
		}
	}()

	// Check if folder exists
	if exists, err := utils.CheckTenderFolderExists("tenderbharat", tenderID); err != nil {
		mu.Lock()
		taskStore[tenderID].Status = StatusFailed
		taskStore[tenderID].Message = err.Error()
		mu.Unlock()
		return
	} else if exists {
		mu.Lock()
		taskStore[tenderID].Status = StatusCompleted
		taskStore[tenderID].Message = "Folder already exists"
		mu.Unlock()
		return
	}

	log.Printf("Starting Tender Docs Download for %s", tenderID)

	// Fix links
	for i := range corrigendumLinks {
		corrigendumLinks[i].Link = strings.ReplaceAll(corrigendumLinks[i].Link, "\\u0026", "&")
	}
	tenderURL = strings.ReplaceAll(tenderURL, "\\u0026", "&")

	baseURL, state, err := utils.GetBaseURLAndState(tenderURL)
	if err != nil {
		mu.Lock()
		taskStore[tenderID].Status = StatusFailed
		taskStore[tenderID].Message = err.Error()
		mu.Unlock()
		return
	}

	// Create session
	sess := session.NewSession(baseURL, state)

	if err := sess.EstablishSession("ActiveTenders"); err != nil {
		mu.Lock()
		taskStore[tenderID].Status = StatusFailed
		taskStore[tenderID].Message = err.Error()
		mu.Unlock()
		return
	}

	// Download docs
	downloader := docdownload.NewDocDownloader(sess, state, log.Default())
	if err := downloader.Run(tenderID, tenderURL, corrigendumLinks); err != nil {
		mu.Lock()
		taskStore[tenderID].Status = StatusFailed
		taskStore[tenderID].Message = err.Error()
		mu.Unlock()
		return
	}

	nitDocs, zipFiles := downloader.GetResults()
	log.Printf("Extracted %d NIT documents and %s zip files", len(nitDocs), zipFiles.DocumentName)

	downloader.Reset()

	mu.Lock()
	taskStore[tenderID].Status = StatusCompleted
	taskStore[tenderID].Message = "AWS upload complete"
	mu.Unlock()
}
