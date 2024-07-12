package ninjacrawler

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log/slog"
)

func (app *Crawler) mustGetClient() *mongo.Client {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	databaseURL := fmt.Sprintf("mongodb://%s:%s@%s:%s",
		app.Config.Env("DB_USERNAME"),
		app.Config.Env("DB_PASSWORD"),
		app.Config.Env("DB_HOST"),
		app.Config.Env("DB_PORT"),
	)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(databaseURL))
	if err != nil {
		panic(err)
	}

	// Check if the connection is established
	err = client.Ping(ctx, nil)
	if err != nil {
		app.Logger.Error("Failed to ping MongoDB: %v", err)
		panic(err)
	}

	return client
}

// dropDatabase drops the specified database.
func (app *Crawler) dropDatabase() error {
	client := app.mustGetClient()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := client.Database(app.Name).Drop(ctx)
	if err != nil {
		app.Logger.Error("Failed to drop database: %v", err)
		return err
	}
	app.Logger.Info("Database dropped successfully")
	return nil
}

// // getCollection returns a collection from the database and ensures unique indexing.
func (app *Crawler) getCollection(collectionName string) *mongo.Collection {
	collection := app.Database(app.Name).Collection(collectionName)
	if !contains(app.preference.ExcludeUniqueUrlEntities, collectionName) {
		app.ensureUniqueIndex(collection)
	}
	return collection
}

// ensureUniqueIndex ensures that the "url" field in the collection has a unique index.
func (app *Crawler) ensureUniqueIndex(collection *mongo.Collection) {
	indexModel := mongo.IndexModel{
		Keys:    bson.M{"url": 1},
		Options: options.Index().SetUnique(true),
	}

	_, err := collection.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		app.Logger.Error("Could not create index: %v", err)
	}
}

// insert inserts multiple URL collections into the database.
func (app *Crawler) insert(model string, urlCollections []UrlCollection, parent string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var documents []interface{}
	for _, urlCollection := range urlCollections {
		urlCollection := UrlCollection{
			Url:       urlCollection.Url,
			Parent:    parent,
			Status:    false,
			Error:     false,
			MetaData:  urlCollection.MetaData,
			Attempts:  0,
			CreatedAt: time.Now(),
			UpdatedAt: nil,
		}
		documents = append(documents, urlCollection)
	}

	opts := options.InsertMany().SetOrdered(false)
	collection := app.getCollection(model)
	_, _ = collection.InsertMany(ctx, documents, opts)
}

// newSite creates a new site collection document in the database.
func (app *Crawler) newSite() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	document := SiteCollection{
		Url:       app.Url,
		BaseUrl:   app.BaseUrl,
		Status:    false,
		Attempts:  0,
		StartedAt: time.Now(),
		EndedAt:   nil,
	}

	collection := app.getCollection(app.GetBaseCollection())
	_, _ = collection.InsertOne(ctx, document)
}

// saveProductDetail saves or updates a product detail document in the database.
func (app *Crawler) saveProductDetail(model string, productDetail *ProductDetail) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := app.getCollection(model)
	if !contains(app.preference.ExcludeUniqueUrlEntities, model) {
		_, err := collection.ReplaceOne(ctx, bson.D{{Key: "url", Value: productDetail.Url}}, productDetail, options.Replace().SetUpsert(true))
		if err != nil {
			app.Logger.Error("Could not save product detail: %v", err)
		}
	} else {
		_, _ = collection.InsertOne(ctx, productDetail)
	}
}

// markAsError marks a URL collection as having encountered an error and updates the database.
func (app *Crawler) markAsError(url string, dbCollection string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var result bson.M

	collection := app.getCollection(dbCollection)
	filter := bson.D{{Key: "url", Value: url}}

	err := collection.FindOne(context.TODO(), filter).Decode(&result)
	if err != nil {
		return fmt.Errorf("Collection Not Found: %v", err)
	}
	timeNow := time.Now()
	attempts := result["attempts"].(int32) + 1
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "error", Value: true},
			{Key: "attempts", Value: attempts},
			{Key: "updated_at", Value: &timeNow},
		}},
	}

	_, err = collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("[%s: => %s] could not mark as Error: Please check this [Error]: %v", dbCollection, url, err)
	}

	return nil
}

// markAsComplete marks a URL collection as having encountered an error and updates the database.
func (app *Crawler) markAsComplete(url string, dbCollection string) error {

	timeNow := time.Now()

	collection := app.getCollection(dbCollection)

	filter := bson.D{{Key: "url", Value: url}}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "status", Value: true},
			{Key: "updated_at", Value: &timeNow},
		}},
	}

	_, err := collection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return fmt.Errorf("[:%s:%s] could not mark as Complete: Please check this [Error]: %v", dbCollection, url, err)
	}
	return nil
}
func (app *Crawler) SyncCurrentPageUrl(url, currentPageUrl string, dbCollection string) error {

	timeNow := time.Now()

	collection := app.getCollection(dbCollection)

	filter := bson.D{{Key: "url", Value: url}}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "current_page_url", Value: currentPageUrl},
			{Key: "updated_at", Value: &timeNow},
		}},
	}

	_, err := collection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return fmt.Errorf("[:%s:%s] could not sync currentpage [Error]: %v", dbCollection, url, err)
	}
	return nil
}

// getUrlsFromCollection retrieves URLs from a collection that meet specific criteria.
func (app *Crawler) getUrlsFromCollection(collection string) []string {
	filterCondition := bson.D{
		{Key: "status", Value: false},
		{Key: "attempts", Value: bson.D{{Key: "$lt", Value: app.engine.MaxRetryAttempts}}},
	}
	return extractUrls(filterData(filterCondition, app.getCollection(collection)))
}

// getUrlCollections retrieves URL collections from a collection that meet specific criteria.
func (app *Crawler) getUrlCollections(collection string) []UrlCollection {
	filterCondition := bson.D{
		{Key: "status", Value: false},
		{Key: "attempts", Value: bson.D{{Key: "$lt", Value: app.engine.MaxRetryAttempts}}},
	}
	return app.filterUrlData(filterCondition, app.getCollection(collection))
}

// filterUrlData retrieves URL collections from a collection based on a filter condition.
func (app *Crawler) filterUrlData(filterCondition bson.D, mongoCollection *mongo.Collection) []UrlCollection {
	findOptions := options.Find().SetLimit(1000)

	cursor, err := mongoCollection.Find(context.TODO(), filterCondition, findOptions)
	if err != nil {
		app.Logger.Error(err.Error())
	}

	var results []UrlCollection
	if err = cursor.All(context.TODO(), &results); err != nil {
		app.Logger.Error(err.Error())
	}

	return results
}

// filterData retrieves documents from a collection based on a filter condition.
func filterData(filterCondition bson.D, mongoCollection *mongo.Collection) []bson.M {
	findOptions := options.Find().SetLimit(1000)

	cursor, err := mongoCollection.Find(context.TODO(), filterCondition, findOptions)
	if err != nil {
		slog.Error(err.Error())
	}

	var results []bson.M
	if err = cursor.All(context.TODO(), &results); err != nil {
		slog.Error(err.Error())
	}

	return results
}

// extractUrls extracts URLs from a list of BSON documents.
func extractUrls(results []bson.M) []string {
	var urls []string
	for _, result := range results {
		if url, ok := result["url"].(string); ok {
			urls = append(urls, url)
		}
	}
	return urls
}

// close closes the MongoDB client connection.
func (app *Crawler) closeClient() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return app.Disconnect(ctx)
}

// getUrlCollections retrieves URL collections from a collection that meet specific criteria.
func (app *Crawler) GetProductDetailCollections(collection string, currentPage int) []ProductDetail {
	pageSize := 10000
	findOptions := options.Find()
	findOptions.SetSkip(int64((currentPage - 1) * pageSize))
	findOptions.SetLimit(int64(pageSize))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cursor, err := app.getCollection(collection).Find(context.TODO(), bson.D{}, findOptions)
	if err != nil {
		app.Logger.Error(err.Error())
	}

	defer cursor.Close(ctx)

	var results []ProductDetail
	if err = cursor.All(context.TODO(), &results); err != nil {
		app.Logger.Error(err.Error())
	}
	return results
}
