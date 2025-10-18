package commands

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	docdownload "github.com/vx6fid/tender-scraper/docDownloads"
	"github.com/vx6fid/tender-scraper/session"
	"github.com/vx6fid/tender-scraper/utils"
	types "github.com/vx6fid/tender-scraper/utils/types"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DocumentConfig struct {
	ID               string
	TenderURL        string
	UpdatedAt        time.Time
	CorrigendumLinks []types.CorrLinks
}

func DownloadDocuments(logger *log.Logger) error {
	// configs := getDocumentDownloadConfig()
	configs, err := getTendersWithCorrigendums("", 0)
	if err != nil {
		return err
	}
	for _, config := range configs {
		if err := processTender(config, logger); err != nil {
			logger.Printf("Failed to process tender %s: %v", config.ID, err)
		}
	}
	return nil
}

// ---------------------- PROCESS FUNCTION ----------------------

func processTender(config DocumentConfig, logger *log.Logger) error {
	if exists, err := utils.CheckTenderFolderExists("tenderbharat", config.ID); err != nil {
		return fmt.Errorf("failed to check if tender folder exists: %w", err)
	} else if exists {
		logger.Printf("Tender folder already exists for %s", config.ID)
		return nil
	}

	logger.Printf("Starting Tender Docs Download for %s", config.ID)

	normalizeLinks(&config)

	baseURL, state, err := utils.GetBaseURLAndState(config.TenderURL)
	if err != nil {
		return fmt.Errorf("failed to get base URL and state: %w", err)
	}

	logger.Printf("BaseURL: %s, State: %s\n", baseURL, state)

	sess := session.NewSession(baseURL, state)
	if err := sess.EstablishSession("ActiveTenders"); err != nil {
		return fmt.Errorf("[%s] failed to establish tender session: %w", state, err)
	}

	downloader := docdownload.NewDocDownloader(sess, state, logger)
	if err := downloader.Run(config.ID, config.TenderURL, config.CorrigendumLinks); err != nil {
		return fmt.Errorf("[%s] doc download failed: %w", state, err)
	}

	nitDocs, zipFiles := downloader.GetResults()
	logger.Printf("Extracted %d NIT documents and %s zip files", len(nitDocs), zipFiles.DocumentName)

	downloader.Reset()
	return nil
}

// ------------------- Helper Functions ---------------------

func getTendersWithCorrigendums(dateStr string, tenderValue int) ([]DocumentConfig, error) {
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

	if tenderValue == 0 {
		tenderValue = 0
	}

	var filterDate time.Time
	if dateStr == "" {
		// Default: one year before today
		now := time.Now()
		oneYearAgo := now.AddDate(-1, 0, 0) // subtract 1 year
		filterDate = time.Date(oneYearAgo.Year(), oneYearAgo.Month(), oneYearAgo.Day(),
			0, 0, 0, 0, time.UTC) // start of that day in UTC
	} else {
		filterDate, err = time.Parse("02012006", dateStr) // ddmmyyyy
		if err != nil {
			return nil, fmt.Errorf("invalid date format (expected ddmmyyyy): %w", err)
		}
	}

	filter := bson.M{
		"tender_value": bson.M{"$gt": tenderValue},
		"updated_at":   bson.M{"$gte": filterDate},
	}

	projection := bson.M{"_id": 1, "corrigendums": 1, "link": 1, "updated_at": 1}

	cur, err := collection.Find(ctx, filter, options.Find().SetProjection(projection))
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer cur.Close(ctx)

	var results []DocumentConfig

	if !cur.Next(ctx) {
		fmt.Println("No documents found")
		return nil, nil
	}

	for cur.Next(ctx) {
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

func getDocumentDownloadConfig() []DocumentConfig {

	return []DocumentConfig{
		{
			ID:        "68695b2955d119428e5086ab",
			TenderURL: "https://etenders.gov.in/eprocure/app?component=%24DirectLink&page=FrontEndLatestActiveCorrigendums&service=direct&session=T&sp=SwKgOCf7CLcX8A0VIcO%2FJUA%3D%3D",
			CorrigendumLinks: []types.CorrLinks{
				{
					Name: "267020.pdf",
					Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_9\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=SXa2%2Bv1L%2FiUoZA9%2FQE0KbZajv0bVL9ByrEDT3Hk8pjFA%3D",
					Type: "Date",
				},
				{
					Name: "C3.pdf",
					Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_9\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=SCFSvWn251d7lvuC4Nh5dGcW686t3AHcBGQneN2DULys%3D",
					Type: "Date",
				},
				{
					Name: "7020.pdf",
					Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_41\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=Sacb93OC9%2B9Kl%2FrMQHu6rDlCS8qgPX7ttE5%2BAX71JLh0%3D",
					Type: "Others",
				},
				{
					Name: "Pkg-III-DOCS.part06.rar",
					Link: "https://etenders.gov.in/eprocure/app?component=%24DirectLink_41\u0026page=CorrViewDetailsPrint\u0026service=direct\u0026session=T\u0026sp=SYBgqgF2rfPSuzM%2F4m3729eUcv4IY4eKw825%2BKxXulQmrRv1rpDQuT0%2F%2Bft9JQhCI",
					Type: "Others",
				},
			},
		},
	}
}

func normalizeLinks(config *DocumentConfig) {
	for i := range config.CorrigendumLinks {
		config.CorrigendumLinks[i].Link = strings.ReplaceAll(config.CorrigendumLinks[i].Link, "\\u0026", "&")
	}
	config.TenderURL = strings.ReplaceAll(config.TenderURL, "\\u0026", "&")
}
