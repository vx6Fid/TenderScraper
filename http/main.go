package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/vx6fid/tender-scraper/utils"
	types "github.com/vx6fid/tender-scraper/utils/types"
)

// func init() {
// 	readSecret := func(path string) string {
// 		data, err := os.ReadFile(path)
// 		if err != nil {
// 			log.Fatalf("Failed to read secret %s: %v", path, err)
// 		}
// 		return strings.TrimSpace(string(data))
// 	}

// 	os.Setenv("AWS_ACCESS_KEY_ID", readSecret("/run/secrets/aws_access_key"))
// 	os.Setenv("AWS_SECRET_ACCESS_KEY", readSecret("/run/secrets/aws_secret_key"))
// 	os.Setenv("AWS_REGION", readSecret("/run/secrets/aws_region"))
// }

func main() {
	LoadEnvOrFatal()

	StartWorkerPool(5) // limit to 5 concurrent downloads

	r := mux.NewRouter()
	r.HandleFunc("/health", healthHandler).Methods("GET")
	r.HandleFunc("/download", startDownloadHandler).Methods("POST")
	r.HandleFunc("/status/{taskID}", statusHandler).Methods("GET")

	log.Println("Server running on :8080")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", r))
}

// Write a health handler
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Server is Up and Running"))
}

func enqueueTenderDownload(tenderID, tenderURL string, corrigendumLinks []types.CorrLinks, baseURLs []types.URLS) error {
	job := func() {
		runTenderDownload(tenderID, tenderURL, corrigendumLinks, baseURLs)
	}

	select {
	case jobQueue <- job:
		return nil
	default:
		return fmt.Errorf("server busy: too many pending downloads")
	}
}

// Start a new tender download
func startDownloadHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenderID         string            `json:"tender_id"`
		TenderURL        string            `json:"tender_url"`
		CorrigendumLinks []types.CorrLinks `json:"corrigendum_links"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := enqueueTenderDownload(req.TenderID, req.TenderURL, req.CorrigendumLinks, utils.BaseURLs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	resp := map[string]string{"task_id": req.TenderID, "status": "queued"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Get task status
func statusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["taskID"]

	mu.RLock()
	defer mu.RUnlock()

	log.Printf("Status requested for taskID: %s", taskID)

	if task, ok := taskStore[taskID]; ok {
		log.Printf("Returning status: %+v", task)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(task)
	} else {
		log.Printf("Task not found: %s", taskID)
		http.Error(w, "Task not found", http.StatusNotFound)
	}
}
