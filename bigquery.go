package ninjacrawler

import (
	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/compute/metadata"
	"context"
	"fmt"
	"os"
	"time"
)

type BigQueryData struct {
	URL       string    `bigquery:"url"`
	HTMLData  string    `bigquery:"html_data"`
	CreatedAt time.Time `bigquery:"created_at"`
}

func (app *Crawler) sendHtmlToBigquery(html, url string) error {
	gcpServiceAccount := app.Config.EnvString("GCP_SERVICE_ACCOUNT", "service-account-stg.json")
	dataset := app.Config.GetString("BIGQUERY_DATASET")
	table := app.Config.GetString("BIGQUERY_TABLE")
	projectID, err := metadata.ProjectID()
	msgStr := fmt.Sprintf("dataset: %s, table: %s, projectID: %s", dataset, table, projectID)
	if err != nil {
		return fmt.Errorf("failed to get project ID: %v", err)
	}

	if err := os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", gcpServiceAccount); err != nil {
		return fmt.Errorf("Failed to set GOOGLE_APPLICATION_CREDENTIALS: %v\n", err)
	}

	// Create context
	ctx := context.Background()

	// Initialize BigQuery client
	client, err := bigquery.NewClient(ctx, projectID)
	if err != nil {
		return fmt.Errorf("failed to create BigQuery client: %v", err)
	}
	defer client.Close()

	// Specify the dataset and table where data will be inserted
	inserter := client.Dataset(dataset).Table(table).Inserter()

	// Prepare the data to insert
	rows := []*BigQueryData{
		{
			URL:       url,
			HTMLData:  html,
			CreatedAt: time.Now(), // This field is used for partitioning
		},
	}

	// Insert data into the table
	if inserterError := inserter.Put(ctx, rows); inserterError != nil {
		return fmt.Errorf("failed to insert data: %v and data: %s", inserterError, msgStr)
	}
	return nil
}
