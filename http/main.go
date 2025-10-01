package main

import (
	"encoding/json"
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
	// if _, err := os.Stat(".env"); err == nil {
	// 	godotenv.Load()
	// }

	r := mux.NewRouter()
	r.HandleFunc("/download", startDownloadHandler).Methods("POST")
	r.HandleFunc("/status/{taskID}", statusHandler).Methods("GET")

	log.Println("Server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
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

	go runTenderDownload(req.TenderID, req.TenderURL, req.CorrigendumLinks, utils.BaseURLs)

	resp := map[string]string{"task_id": req.TenderID, "status": "started"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Get task status
func statusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["taskID"]

	mu.Lock()
	defer mu.Unlock()

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
