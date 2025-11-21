package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"
)

// OutboxMessage is used for scanning rows.
type OutboxMessage struct {
	ID          int64
	JobID       string
	Destination string
	Payload     []byte
}

// PublishFunc is the function signature used to publish to RabbitMQ.
type PublishFunc func(destination string, payload []byte) error

// startOutboxPublisher runs an infinite loop that:
// 1. Reads outbox rows where sent=false
// 2. Publishes to RabbitMQ via publish()
// 3. Marks them as sent=true
//
// This MUST run in a separate goroutine from main().
func startOutboxPublisher(ctx context.Context, publish PublishFunc) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	log.Println("[outbox] publisher started")

	for {
		select {
		case <-ctx.Done():
			log.Println("[outbox] stopping publisher")
			return

		case <-ticker.C:
			processOutboxBatch(ctx, publish)
		}
	}
}

// processOutboxBatch processes up to 10 unsent outbox events.
func processOutboxBatch(ctx context.Context, publish PublishFunc) {
	rows, err := db.Query(ctx,
		`SELECT id, job_id, destination, payload
		 FROM outbox
		 WHERE sent = false
		 ORDER BY id ASC
		 LIMIT 10`)
	if err != nil {
		log.Printf("[outbox] query error: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var msg OutboxMessage
		if err := rows.Scan(&msg.ID, &msg.JobID, &msg.Destination, &msg.Payload); err != nil {
			log.Printf("[outbox] scan error: %v", err)
			continue
		}

		if err := publish(msg.Destination, msg.Payload); err != nil {
			// Publish failed → increment attempts
			_, _ = db.Exec(ctx, `UPDATE outbox SET attempts = attempts + 1 WHERE id=$1`, msg.ID)
			log.Printf("[outbox] publish failed (id=%d): %v", msg.ID, err)
			continue
		}

		// Mark msg as sent; idempotent
		_, err := db.Exec(ctx,
			`UPDATE outbox
			 SET sent=true, sent_at=now()
			 WHERE id=$1 AND sent=false`,
			msg.ID)
		if err != nil {
			log.Printf("[outbox] mark-sent error (id=%d): %v", msg.ID, err)
		} else {
			log.Printf("[outbox] delivered job_id=%s → %s", msg.JobID, msg.Destination)
		}
	}

	if rows.Err() != nil {
		log.Printf("[outbox] row error: %v", rows.Err())
	}
}

// Example publisher for debugging without RabbitMQ.
// Replace this with real RabbitMQ later in Phase C.
func DebugLogPublisher(dest string, payload []byte) error {
	var m map[string]any
	_ = json.Unmarshal(payload, &m)

	log.Printf("[outbox->%s] mock publish: %+v", dest, m)
	return nil
}

// Example error publisher for testing retries
func AlwaysFailPublisher(dest string, payload []byte) error {
	return errors.New("intentional failure for testing")
}
