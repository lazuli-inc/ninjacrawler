package ninjacrawler

import (
	"cloud.google.com/go/datastore"
	"context"
	"fmt"
	"time"
)

func (app *Crawler) getDataStoreClient() *datastore.Client {
	ctx := context.Background()

	// Replace MongoDB connection logic with Datastore connection
	client, err := datastore.NewClient(ctx, app.Config.EnvString("PROJECT_ID"))
	if err != nil {
		app.Logger.Error("Failed to create Datastore client: %v", err)
		panic(err)
	}

	return client
}
func (app *Crawler) InsertUrlCollectionsDS(model string, urlCollections []UrlCollection, parent string) {
	ctx := context.Background()
	client := app.getDataStoreClient()
	defer client.Close()

	// Datastore requires you to define keys for each entity
	var keys []*datastore.Key
	var entities []*UrlCollection

	for _, urlCollection := range urlCollections {
		// Create a new Datastore key using the model and the URL as a unique identifier
		key := datastore.NameKey(model, urlCollection.Url, nil)
		keys = append(keys, key)
		// Update the URL collection with the appropriate fields
		urlCollection.Parent = parent
		urlCollection.Status = false
		urlCollection.Error = false
		urlCollection.Attempts = 0
		urlCollection.CreatedAt = time.Now()
		entities = append(entities, &urlCollection)
	}

	// Use PutMulti to insert multiple entities
	_, err := client.PutMulti(ctx, keys, entities)
	if err != nil {
		app.Logger.Error("Failed to insert URL collections: %v", err)
	}
}
func (app *Crawler) getUrlCollectionsDS(collection string) []UrlCollection {
	ctx := context.Background()
	client := app.getDataStoreClient()
	defer client.Close()

	var urlCollections []UrlCollection
	query := datastore.NewQuery(collection).
		Filter("Status =", false).
		Filter("Attempts <", app.engine.MaxRetryAttempts).
		Limit(dbFilterLimit)

	_, err := client.GetAll(ctx, query, &urlCollections)
	if err != nil {
		app.Logger.Error("Failed to retrieve URL collections: %v", err)
	}

	return urlCollections
}
func (app *Crawler) markAsCompleteDS(url string, dbCollection string) error {
	ctx := context.Background()
	client := app.getDataStoreClient()
	defer client.Close()

	// Get the entity by its key (URL)
	key := datastore.NameKey(dbCollection, url, nil)
	var urlCollection UrlCollection
	if err := client.Get(ctx, key, &urlCollection); err != nil {
		return fmt.Errorf("could not retrieve entity: %v", err)
	}

	timeNow := time.Now()
	// Update the fields
	urlCollection.Status = true
	urlCollection.UpdatedAt = &timeNow

	// Save the updated entity
	if _, err := client.Put(ctx, key, &urlCollection); err != nil {
		return fmt.Errorf("could not mark as complete: %v", err)
	}

	return nil
}
