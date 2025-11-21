package commands

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	docdownload "github.com/vx6fid/tender-scraper/docDownloads"
	"github.com/vx6fid/tender-scraper/session"
	"github.com/vx6fid/tender-scraper/utils"
	types "github.com/vx6fid/tender-scraper/utils/types"

	"go.mongodb.org/mongo-driver/bson"
	primitive "go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DocumentConfig struct {
	ID               string
	TenderURL        string
	UpdatedAt        time.Time
	CorrigendumLinks []types.CorrLinks
}

// --- Stats structs (keep these in a common package/file) ---
type DownloadStats struct {
	TenderID string
	State    string
	Existing bool
	Total    int
	Success  int
	Skipped  int
	Failed   int
}

type BatchStats struct {
	Total   int64
	Success int64
	Skipped int64
	Failed  int64
}

func (b *BatchStats) Add(ds DownloadStats) {
	atomic.AddInt64(&b.Total, int64(ds.Total))
	atomic.AddInt64(&b.Success, int64(ds.Success))
	atomic.AddInt64(&b.Skipped, int64(ds.Skipped))
	atomic.AddInt64(&b.Failed, int64(ds.Failed))
}

// --- Entry point (concurrent worker pool) ---
func DownloadDocuments(logger *log.Logger) error {
	configs, err := getTendersWithCorrigendums(500000000)
	if err != nil {
		return err
	}

	sem := make(chan struct{}, utils.MaxDownloadWorkers)
	var wg sync.WaitGroup

	var completed int64
	total := int64(len(configs))
	batch := &BatchStats{}

	for _, config := range configs {
		cfg := config
		sem <- struct{}{}
		wg.Add(1)

		go func() {
			defer func() { <-sem; wg.Done() }()
			ctx := context.Background()
			ds, err := ProcessTender(ctx, cfg, logger)
			if err != nil {
				// keep error reporting but still aggregate the stats
				logger.Printf("[%s] error: %v", cfg.ID, err)
			}
			batch.Add(ds)

			count := atomic.AddInt64(&completed, 1)
			if count%10 == 0 || count == total {
				fmt.Println()
				logger.Printf("[Progress] : %d/%d\n", count, total)
				fmt.Println()
			}
		}()
	}

	wg.Wait()

	logger.Printf("Batch Summary: [Docs: total=%d, success=%d, skipped=%d, failed=%d]",
		batch.Total, batch.Success, batch.Skipped, batch.Failed)

	return nil
}

// ---------------------- PROCESS FUNCTION ----------------------

// processTender returns per-tender DownloadStats and an error (if any)
func ProcessTender(context context.Context, config DocumentConfig, logger *log.Logger) (DownloadStats, error) {
	// Determine if folder exists -> skipWorkNit
	skipWorkNit := false
	exists, err := utils.CheckTenderFolderExists(context, "tenderbharat-ap-south-1", config.ID)
	if err != nil {
		return DownloadStats{}, fmt.Errorf("failed to check tender folder: %w", err)
	}
	if exists {
		// logger.Printf("Work Item and NIT Docs of %s already exists", config.ID)
		skipWorkNit = true
	}

	normalizeLinks(&config)

	baseURL, state, err := utils.GetBaseURLAndState(config.TenderURL)
	if err != nil {
		return DownloadStats{}, fmt.Errorf("failed to get base URL and state: %w", err)
	}

	sess := session.NewSession(baseURL, state)
	if err := sess.EstablishSession(context, "ActiveTenders"); err != nil {
		return DownloadStats{}, fmt.Errorf("[%s] failed to establish tender session: %w", state, err)
	}

	downloader := docdownload.NewDocDownloader(sess, state, logger, skipWorkNit)
	if err := downloader.Run(context, config.ID, config.TenderURL, config.CorrigendumLinks); err != nil {
		// On Run error we will compute counts from whatever was populated and mark as failed per your rule.
		// Don't return early — create stats to log & aggregate, but return the error as well.
		nitCount := len(downloader.NITDocs)
		// corrCount := len(downloader.CorrigendumDocs)
		zipCount := 0
		if downloader.WorkItemZip.URL != "" {
			zipCount = 1
		}
		totalDocs := nitCount + len(config.CorrigendumLinks) + zipCount

		stats := DownloadStats{
			TenderID: config.ID,
			State:    state,
			Existing: skipWorkNit,
			Total:    totalDocs,
			Failed:   totalDocs, // per rule: mark all as failed when Run errors
		}

		// reset downloader if lock/cleanup needed (assumes Reset exists)
		if r := tryDownloaderReset(downloader); r != nil {
			logger.Printf("[%s] downloader reset failed: %v", state, r)
		}

		// Log the per-tender line
		existingStr := "N"
		if stats.Existing {
			existingStr = "Y"
		}
		logger.Printf("[%s|%s][Existing=%s] [Docs: total=%d, success=%d, skipped=%d, failed=%d]",
			stats.State, stats.TenderID,
			existingStr,
			stats.Total, stats.Success, stats.Skipped, stats.Failed)

		return stats, fmt.Errorf("[%s] doc download failed: %w", state, err)
	}

	// If Run succeeded, compute counts deterministically from downloader fields
	nitCount := len(downloader.NITDocs)
	corrCount := len(downloader.CorrigendumDocs)
	zipCount := 0
	if downloader.WorkItemZip.URL != "" {
		zipCount = 1
	}
	totalDocs := nitCount + len(config.CorrigendumLinks) + zipCount
	successCount := nitCount + corrCount + zipCount

	var stats DownloadStats
	if skipWorkNit {
		// NIT + WorkItem are skipped, Corrigenda are processed freshly.
		skippedCount := nitCount + zipCount
		successCount = corrCount

		stats = DownloadStats{
			TenderID: config.ID,
			State:    state,
			Existing: true,
			Total:    totalDocs,
			Success:  successCount,
			Skipped:  skippedCount,
		}
	} else {
		stats = DownloadStats{
			TenderID: config.ID,
			State:    state,
			Existing: false,
			Total:    totalDocs,
			Success:  successCount,
		}
	}

	// Reset downloader internal state if needed
	if r := tryDownloaderReset(downloader); r != nil {
		logger.Printf("[%s] downloader reset failed: %v", state, r)
	}

	// Log concise per-tender line
	existingStr := "N"
	if stats.Existing {
		existingStr = "Y"
	}
	logger.Printf("[%s|%s][Existing=%s] [Docs: total=%d, success=%d, skipped=%d, failed=%d]",
		stats.State, stats.TenderID,
		existingStr,
		stats.Total, stats.Success, stats.Skipped, stats.Failed)

	return stats, nil
}

// Helper to reset downloader if method exists; returns nil on success or an error
func tryDownloaderReset(d *docdownload.DocDownloader) error {
	// if Reset exists, call it; otherwise return nil.
	// Replace with the actual reset call if different.
	type resetter interface {
		Reset()
	}
	if r, ok := any(d).(resetter); ok {
		r.Reset()
		return nil
	}
	return nil
}

// ------------------- Helper Functions ---------------------

func getTendersWithCorrigendums(tenderValue int) ([]DocumentConfig, error) {
	MongoURI := os.Getenv("MONGO_CONN_STRING")
	DBName := os.Getenv("DB_NAME")
	CollectionName := os.Getenv("COLLECTION_NAME")

	if MongoURI == "" || DBName == "" || CollectionName == "" {
		log.Fatal("Missing required environment variables for MongoDB")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clientOpts := options.Client().ApplyURI(MongoURI)
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(DBName)
	collection := db.Collection(CollectionName)

	filter := bson.M{
		"tender_value": bson.M{"$gte": tenderValue},
	}
	// id, err := primitive.ObjectIDFromHex("6907a9d8dbd1f721dcd87a9f")
	if err != nil {
		log.Fatal(err)
	}
	// fmt.Println("Starting Filters")

	// filter := bson.M{"_id": id}

	projection := bson.M{"_id": 1, "location": 1, "corrigendums": 1, "link": 1, "updated_at": 1}

	cur, err := collection.Find(ctx, filter, options.Find().SetProjection(projection))
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer cur.Close(ctx)

	var results []DocumentConfig

	// if !cur.Next(ctx) {
	// 	fmt.Println("No documents found")
	// 	return nil, nil
	// }

	for cur.Next(ctx) {
		// fmt.Println("Processing Document")
		var doc struct {
			ID           any       `bson:"_id"`
			Corrigendums []bson.M  `bson:"corrigendums"`
			UpdatedAt    time.Time `bson:"updated_at"`
			Link         string    `bson:"link"`
		}

		if err := cur.Decode(&doc); err != nil {
			log.Printf("failed to decode document: %v", err)
			continue
		}

		tenderID := ""
		if oid, ok := doc.ID.(primitive.ObjectID); ok {
			tenderID = oid.Hex()
		} else {
			tenderID = fmt.Sprintf("%v", doc.ID) // fallback
		}

		var corrigendumLinks []types.CorrLinks
		tenderURL := doc.Link
		for _, corr := range doc.Corrigendums {
			corrType, _ := corr["Type"].(string)
			if corr["Details"] == nil {
				continue
			}

			details, ok := corr["Details"].(primitive.A)
			if !ok {
				continue
			}

			for _, d := range details {
				if m, ok := d.(bson.M); ok {
					name, _ := m["DocumentName"].(string)
					link, _ := m["DocumentLink"].(string)
					if name != "" && link != "" {
						corrigendumLinks = append(corrigendumLinks, types.CorrLinks{
							Name: name,
							Link: link,
							Type: corrType,
						})
					}
				}
			}
		}

		results = append(results, DocumentConfig{
			ID:               tenderID,
			TenderURL:        tenderURL,
			UpdatedAt:        doc.UpdatedAt,
			CorrigendumLinks: corrigendumLinks,
		})
	}

	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}
	// fmt.Println(results)
	return results, nil
}

func normalizeLinks(config *DocumentConfig) {
	for i := range config.CorrigendumLinks {
		config.CorrigendumLinks[i].Link = strings.ReplaceAll(config.CorrigendumLinks[i].Link, "\\u0026", "&")
	}
	config.TenderURL = strings.ReplaceAll(config.TenderURL, "\\u0026", "&")
}
