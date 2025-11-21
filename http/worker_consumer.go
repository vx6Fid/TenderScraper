package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/vx6fid/tender-scraper/cli/commands"
)

// Configuration
const (
	rabbitPrefetchCount = 5   // per channel prefetch (concurrency tuning)
	workerConcurrency   = 5   // number of goroutines processing deliveries
	maxAttempts         = 5   // fail job after this many DB attempts
	jobTimeoutSeconds   = 600 // worker-side job timeout (10 minutes)
)

// Rabbit wrapper struct
type Rabbit struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	url     string
}

// Start a persistent rabbit connection
func newRabbit(url string) (*Rabbit, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}
	// QoS (prefetch) — limit unacked messages per consumer
	if err := ch.Qos(rabbitPrefetchCount, 0, false); err != nil {
		conn.Close()
		return nil, err
	}
	return &Rabbit{conn: conn, channel: ch, url: url}, nil
}

func (r *Rabbit) Close() {
	if r.channel != nil {
		_ = r.channel.Close()
	}
	if r.conn != nil {
		_ = r.conn.Close()
	}
}

// startGoWorkerConsumer connects to RabbitMQ queue "jobs.go" and starts consuming.
func startGoWorkerConsumer(ctx context.Context, rabbitURL string) error {
	r, err := newRabbit(rabbitURL)
	if err != nil {
		return err
	}

	// Ensure queue exists (durable)
	_, err = r.channel.QueueDeclare(
		"jobs.go", // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // args
	)
	if err != nil {
		r.Close()
		return err
	}

	msgs, err := r.channel.Consume(
		"jobs.go", // queue
		"",        // consumer
		false,     // autoAck=false -> we ack manually
		false,     // exclusive
		false,     // noLocal
		false,     // noWait
		nil,       // args
	)
	if err != nil {
		r.Close()
		return err
	}

	log.Println("[worker] Go consumer started")

	// Worker pool to process deliveries concurrently
	var wg sync.WaitGroup
	deliveries := make(chan amqp.Delivery)

	// Start worker goroutines
	for i := range workerConcurrency {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for d := range deliveries {
				_ = processDeliveryWithRetry(ctx, &d, r.channel)
			}
		}(i)
	}

	// Monitor cancellations and drain
	go func() {
		<-ctx.Done()
		// stop receiving and drain
		log.Println("[worker] context canceled, stopping consumer")
		_ = r.channel.Cancel("", false)
		close(deliveries)
	}()

	// Feed deliveries channel
	for d := range msgs {
		select {
		case deliveries <- d:
			// passed to worker
		case <-ctx.Done():
			// cancel: reject the message so it can be requeued
			_ = d.Nack(false, true)
			break
		}
	}

	// Wait all workers finish
	wg.Wait()
	r.Close()
	return nil
}

// processDeliveryWithRetry handles a single delivery and ensures proper ack/nack.
// It unmarshals message (expects {"job_id":"..."}), then calls processGoMessage.
// If processing fails for transient reasons, Nack with requeue true to allow redelivery.
// If job reached maxAttempts or permanent failure, ack and mark job failed.
func processDeliveryWithRetry(ctx context.Context, d *amqp.Delivery, ch *amqp.Channel) error {
	// Quick parse
	var msg map[string]any
	if err := json.Unmarshal(d.Body, &msg); err != nil {
		log.Printf("[worker] invalid message body: %v", err)
		_ = d.Ack(false) // discard malformed message
		return err
	}
	jobID, ok := msg["job_id"].(string)
	if !ok || jobID == "" {
		log.Printf("[worker] invalid job_id in message")
		_ = d.Ack(false)
		return errors.New("invalid job_id")
	}

	// process
	err := processGoMessage(ctx, jobID)
	if err == nil {
		// success -> ack
		if err := d.Ack(false); err != nil {
			log.Printf("[worker] ack error: %v", err)
		}
		return nil
	}

	// At this point processing failed. Determine whether to retry or mark failed.
	log.Printf("[worker] processing job=%s failed: %v", jobID, err)

	// Fetch current attempts from DB to decide
	var attempts int
	errScan := db.QueryRow(ctx, `SELECT attempts FROM jobs WHERE id=$1`, jobID).Scan(&attempts)
	if errScan != nil {
		// if we can't read attempts, nack with requeue true
		_ = d.Nack(false, true)
		return err
	}

	if attempts >= maxAttempts {
		// mark failed permanently and ack message (do not requeue)
		_, _ = db.Exec(ctx, `UPDATE jobs SET status='failed', last_error=$1, updated_at=now() WHERE id=$2`, err.Error(), jobID)
		_ = d.Ack(false)
		log.Printf("[worker] job=%s marked failed after %d attempts", jobID, attempts)
		return err
	}

	// For transient failures, nack with requeue true so RabbitMQ will redeliver after a short delay.
	_ = d.Nack(false, true)
	return err
}

// processGoMessage: claim job in DB, run Go stage, update job and insert outbox for python stage
func processGoMessage(ctx context.Context, jobID string) error {
	// Claim the job: only succeed if status = 'pending'
	// Also increment attempts.
	var payloadBytes []byte
	var curAttempts int
	err := db.QueryRow(ctx, `
		UPDATE jobs
		SET status='running_go', attempts = attempts + 1, updated_at = now()
		WHERE id=$1 AND status='pending'
		RETURNING payload, attempts
	`, jobID).Scan(&payloadBytes, &curAttempts)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || err == sql.ErrNoRows {
			// Not pending -> nothing to do (maybe already claimed)
			return nil
		}
		return err
	}

	// Unmarshal payload
	var jobPayload JobPayload
	if err := json.Unmarshal(payloadBytes, &jobPayload); err != nil {
		// write last_error and set failed
		_, _ = db.Exec(ctx, `UPDATE jobs SET status='failed', last_error=$1, updated_at=now() WHERE id=$2`, err.Error(), jobID)
		return err
	}

	// Run the actual heavy work with a worker-controlled timeout
	wctx, cancel := context.WithTimeout(ctx, time.Duration(jobTimeoutSeconds)*time.Second)
	defer cancel()

	stats, err := commands.ProcessTender(wctx, commands.DocumentConfig{
		ID:               jobPayload.TenderID,
		TenderURL:        jobPayload.TenderURL,
		CorrigendumLinks: jobPayload.CorrigendumLinks,
		UpdatedAt:        time.Now(),
	}, log.Default())

	// Prepare go_result payload (structured)
	goResult := map[string]any{
		"stats": stats,
	}
	if err != nil {
		goResult["error"] = err.Error()
	}

	goBytes, _ := json.Marshal(goResult)

	// Start transaction to update job and insert outbox for python stage
	tx, txErr := db.Begin(ctx)
	if txErr != nil {
		return txErr
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// If ProcessTender failed, we still push to python stage (policy decision).
	// Here: if a transient failure, we may choose to set status back to 'pending' to retry.
	// Simpler: on success or partial success, set status to 'pending_python' and insert outbox.
	// On fatal errors, set 'failed'. We'll treat any returned error as recoverable and still enqueue python.
	if err != nil {
		// if attempts exceeded, mark failed
		if curAttempts >= maxAttempts {
			if _, e := tx.Exec(ctx, `UPDATE jobs SET status='failed', go_result=$1::jsonb, last_error=$2, updated_at=now() WHERE id=$3`, goBytes, err.Error(), jobID); e != nil {
				_ = tx.Rollback(ctx)
				return e
			}
			if e := tx.Commit(ctx); e != nil {
				return e
			}
			return err
		}
		// else: write go_result and still enqueue python for post-processing/inspection
		_, e := tx.Exec(ctx, `UPDATE jobs SET go_result=$1::jsonb, status='pending_python', updated_at=now() WHERE id=$2`, goBytes, jobID)
		if e != nil {
			_ = tx.Rollback(ctx)
			return e
		}
		// insert outbox for python stage
		outPayload := map[string]any{"job_id": jobID}
		outB, _ := json.Marshal(outPayload)
		_, e = tx.Exec(ctx, `INSERT INTO outbox (job_id, destination, payload) VALUES ($1, 'jobs.python', $2)`, jobID, outB)
		if e != nil {
			_ = tx.Rollback(ctx)
			return e
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		return err
	}

	// Success path: write go_result and enqueue python
	_, e := tx.Exec(ctx, `UPDATE jobs SET go_result=$1::jsonb, status='pending_python', updated_at=now() WHERE id=$2`, goBytes, jobID)
	if e != nil {
		_ = tx.Rollback(ctx)
		return e
	}
	outPayload := map[string]interface{}{"job_id": jobID}
	outB, _ := json.Marshal(outPayload)
	_, e = tx.Exec(ctx, `INSERT INTO outbox (job_id, destination, payload) VALUES ($1, 'jobs.python', $2)`, jobID, outB)
	if e != nil {
		_ = tx.Rollback(ctx)
		return e
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}
