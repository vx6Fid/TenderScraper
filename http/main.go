package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/vx6fid/tender-scraper/utils"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

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
		CorrigendumLinks []utils.CorrLinks `json:"corrigendum_links"`
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

	if task, ok := taskStore[taskID]; ok {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(task)
	} else {
		http.Error(w, "Task not found", http.StatusNotFound)
	}
}
