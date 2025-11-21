package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	types "github.com/vx6fid/tender-scraper/utils/types"
)

// GLOBAL DB CONNECTION
var db *pgxpool.Pool

// -------------------------------
// DB INITIALIZATION
// -------------------------------
func initDB(ctx context.Context, dsn string) error {
	conn, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}

	db = conn
	return nil
}

// -------------------------------
// JOB PAYLOAD STRUCT
// -------------------------------
type JobPayload struct {
	TenderID         string            `json:"tender_id"`
	TenderURL        string            `json:"tender_url"`
	CorrigendumLinks []types.CorrLinks `json:"corrigendum_links"`
}

// -------------------------------
// CREATE JOB + OUTBOX ENTRY
// -------------------------------
func createJobAndEnqueue(ctx context.Context, payload JobPayload) (string, error) {
	payloadBytes, _ := json.Marshal(payload)

	tx, err := db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	// Insert job row
	var jobID string
	err = tx.QueryRow(ctx,
		`INSERT INTO jobs (payload, status)
		 VALUES ($1, 'pending')
		 RETURNING id`,
		payloadBytes,
	).Scan(&jobID)
	if err != nil {
		return "", err
	}

	// Insert outbox entry for Go stage
	outboxPayload := map[string]any{
		"job_id":     jobID,
		"created_at": time.Now(),
	}
	outBytes, _ := json.Marshal(outboxPayload)

	_, err = tx.Exec(ctx,
		`INSERT INTO outbox (job_id, destination, payload)
		 VALUES ($1, 'jobs.go', $2)`,
		jobID, outBytes,
	)
	if err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}

	return jobID, nil
}

// -------------------------------
// HTTP HANDLERS
// -------------------------------
func downloadHandler(w http.ResponseWriter, r *http.Request) {
	var req JobPayload

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.TenderID == "" || req.TenderURL == "" {
		http.Error(w, "tender_id and tender_url are required", http.StatusBadRequest)
		return
	}

	jobID, err := createJobAndEnqueue(r.Context(), req)
	if err != nil {
		log.Println("enqueue error:", err)
		http.Error(w, "failed to enqueue job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"job_id": jobID,
	})
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	jobID := mux.Vars(r)["taskID"]

	var status string
	var payloadBytes []byte
	var goBytes, pyBytes []byte
	var lastErr *string

	err := db.QueryRow(r.Context(),
		`SELECT status, payload, go_result, python_result, last_error
		 FROM jobs WHERE id=$1`,
		jobID,
	).Scan(&status, &payloadBytes, &goBytes, &pyBytes, &lastErr)

	if err != nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	var payload JobPayload
	_ = json.Unmarshal(payloadBytes, &payload)

	var goRes any
	if len(goBytes) > 0 {
		_ = json.Unmarshal(goBytes, &goRes)
	}

	var pyRes any
	if len(pyBytes) > 0 {
		_ = json.Unmarshal(pyBytes, &pyRes)
	}

	response := map[string]any{
		"job_id":        jobID,
		"status":        status,
		"payload":       payload,
		"go_result":     goRes,
		"python_result": pyRes,
		"last_error":    lastErr,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}

// -------------------------------
// MAIN
// -------------------------------
func main() {
	LoadEnvOrFatal()

	// GLOBAL root context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- 1. INIT DB ---
	if err := initDB(ctx, os.Getenv("POSTGRES_CONN_STRING")); err != nil {
		log.Fatalf("failed to connect to DB: %v", err)
	}
	defer db.Close()

	// --- 2. INIT RABBITMQ PUBLISHER ---
	rabbitURL := os.Getenv("RABBITMQ_URL")
	publisher, err := NewRabbitPublisher(rabbitURL)
	if err != nil {
		log.Fatalf("failed to connect RabbitMQ: %v", err)
	}
	defer publisher.Close()

	// --- 3. START OUTBOX PUBLISHER ---
	go startOutboxPublisher(ctx, publisher.Publish)

	// --- 4. START GO WORKER CONSUMER ---
	go func() {
		if err := startGoWorkerConsumer(ctx, rabbitURL); err != nil {
			log.Fatalf("go worker consumer stopped: %v", err)
		}
	}()

	// --- 5. START HTTP SERVER ---
	r := mux.NewRouter()
	r.HandleFunc("/health", healthHandler).Methods("GET")
	r.HandleFunc("/download", downloadHandler).Methods("POST")
	r.HandleFunc("/status/{taskID}", statusHandler).Methods("GET")

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	go func() {
		log.Println("Server running on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	// --- 6. GRACEFUL SHUTDOWN ---
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down...")
	cancel()

	// Shutdown HTTP
	ctxTimeout, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	_ = srv.Shutdown(ctxTimeout)

	log.Println("Shutdown complete.")
}
