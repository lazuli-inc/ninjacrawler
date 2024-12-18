package ninjacrawler

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (app *Crawler) mustGetClient() *mongo.Client {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	databaseURL := fmt.Sprintf("mongodb://%s:%s@%s:%s",
		app.Config.Env("DB_USERNAME"),
		app.Config.Env("DB_PASSWORD"),
		app.Config.Env("DB_HOST"),
		app.Config.Env("DB_PORT"),
	)
	clientOptions := options.Client().
		ApplyURI(databaseURL).
		SetMaxPoolSize(uint64(app.engine.ConcurrentLimit * 2)).
		SetServerSelectionTimeout(10 * time.Second)

	client, err := mongo.Connect(ctx, clientOptions)
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
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
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
	// Create a slice for the index keys, starting with the default 'url'
	indexKeys := bson.D{{Key: "url", Value: 1}}

	// Add additional unique fields, avoiding duplicates
	if app.CurrentProcessorConfig.CollectionIndex != nil {
		for _, field := range *app.CurrentProcessorConfig.CollectionIndex {
			if field != "url" { // Skip if it's already 'url'
				indexKeys = append(indexKeys, bson.E{Key: field, Value: 1})
			}
		}
	}

	// Create the index model
	indexModel := mongo.IndexModel{
		Keys:    indexKeys, // Use bson.D for ordered field key-value pairs
		Options: options.Index().SetUnique(true),
	}

	// Create the index on the collection
	_, err := collection.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		app.Logger.Error("Could not create index: %v", err)
	}
}

// insert inserts multiple URL collections into the database.
func (app *Crawler) insert(model string, urlCollections []UrlCollection, parent string) {
	app.InsertUrlCollections(model, urlCollections, parent)
}
func (app *Crawler) InsertUrlCollections(model string, urlCollections []UrlCollection, parent string) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	var documents []interface{}
	for _, urlCollection := range urlCollections {
		urlCollection := UrlCollection{
			Url:       urlCollection.Url,
			ApiUrl:    urlCollection.ApiUrl,
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
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
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
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
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

// MarkAsError marks a URL collection as having encountered an error and updates the database.
func (app *Crawler) MarkAsError(url string, dbCollection string, errStr string, attempt ...int32) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
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
	if attempt != nil {
		attempts = attempt[0]
	}
	var currentErrLog string
	if result["error_log"] != nil {
		currentErrLog = result["error_log"].(string)
	}

	errStr = currentErrLog + "\n--------\n" + errStr
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "error", Value: true},
			{Key: "error_log", Value: errStr},
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
func (app *Crawler) MarkAsMaxErrorAttempt(url string, dbCollection, errStr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	var result bson.M

	collection := app.getCollection(dbCollection)
	filter := bson.D{{Key: "url", Value: url}}

	err := collection.FindOne(context.TODO(), filter).Decode(&result)
	if err != nil {
		return fmt.Errorf("Collection Not Found: %v", err)
	}
	timeNow := time.Now()
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "error", Value: true},
			{Key: "error_log", Value: errStr},
			{Key: "MaxRetryAttempts", Value: true},
			{Key: "attempts", Value: app.engine.MaxRetryAttempts},
			{Key: "updated_at", Value: &timeNow},
		}},
	}

	_, err = collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("[%s: => %s] could not mark as Error: Please check this [Error]: %v", dbCollection, url, err)
	}
	return nil
}
func (app *Crawler) updateStatusCode(url string, value int) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	var result bson.M

	collection := app.getCollection(app.CurrentCollection)
	filter := bson.D{{Key: "url", Value: url}}

	err := collection.FindOne(context.TODO(), filter).Decode(&result)
	if err != nil {
		return fmt.Errorf("Collection Not Found: %v", err)
	}
	timeNow := time.Now()
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "status_code", Value: value},
			{Key: "updated_at", Value: &timeNow},
		}},
	}

	_, err = collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("[%s: => %s] could not mark as Error: Please check this [Error]: %v", app.CurrentCollection, url, err)
	}
	return nil
}
func (app *Crawler) updateRedirection(url string, redirectedUrl string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	var result bson.M

	collection := app.getCollection(app.CurrentCollection)
	filter := bson.D{{Key: "url", Value: url}}

	err := collection.FindOne(context.TODO(), filter).Decode(&result)
	if err != nil {
		return fmt.Errorf("Collection Not Found: %v", err)
	}
	timeNow := time.Now()
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "redirected_url", Value: redirectedUrl},
			{Key: "updated_at", Value: &timeNow},
		}},
	}

	_, err = collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("[%s: => %s] could not mark as Error: Please check this [Error]: %v", app.CurrentCollection, url, err)
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
func (app *Crawler) markAsBigQueryFailed(url string, errStr string) error {

	timeNow := time.Now()

	collection := app.getCollection(app.CurrentCollection)

	filter := bson.D{{Key: "url", Value: url}}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "bigquery_error", Value: errStr},
			{Key: "updated_at", Value: &timeNow},
		}},
	}

	_, err := collection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return fmt.Errorf("[:%s:%s] could markAsBigQueryFailed: Please check this [Error]: %v", app.CurrentCollection, url, err)
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
func (app *Crawler) getRetryableUrlCollections(collection string) []UrlCollection {
	filterCondition := bson.D{
		{Key: "status", Value: false},
		{Key: "error", Value: true},
		{Key: "attempts", Value: bson.D{{Key: "$gte", Value: 1}}},
	}
	return app.filterUrlData(filterCondition, app.getCollection(collection))
}

// filterUrlData retrieves URL collections from a collection based on a filter condition.
func (app *Crawler) filterUrlData(filterCondition bson.D, mongoCollection *mongo.Collection) []UrlCollection {
	findOptions := options.Find().SetLimit(dbFilterLimit)

	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	cursor, err := mongoCollection.Find(ctx, filterCondition, findOptions)
	if err != nil {
		app.Logger.Error("Failed to retrieve URL data: " + err.Error())
	}
	defer cursor.Close(ctx)

	var results []UrlCollection
	if err := cursor.All(ctx, &results); err != nil {
		app.Logger.Error("Failed to decode URL data: " + err.Error())
	}

	return results
}

// filterData retrieves documents from a collection based on a filter condition.
func filterData(filterCondition bson.D, mongoCollection *mongo.Collection) []bson.M {
	findOptions := options.Find().SetLimit(dbFilterLimit)

	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	cursor, err := mongoCollection.Find(ctx, filterCondition, findOptions)
	if err != nil {
		fmt.Println("Failed to retrieve data: " + err.Error())
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		fmt.Println("Failed to decode data: " + err.Error())
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
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	return app.Disconnect(ctx)
}

// getUrlCollections retrieves URL collections from a collection that meet specific criteria.
func (app *Crawler) GetProductDetailCollections(collection string, currentPage int) []ProductDetail {
	pageSize := 10000 // can be passed as a parameter for flexibility
	findOptions := options.Find().
		SetSkip(int64((currentPage - 1) * pageSize)).
		SetLimit(int64(pageSize))

	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	cursor, err := app.getCollection(collection).Find(ctx, bson.D{}, findOptions)
	if err != nil {
		app.Logger.Error("Failed to retrieve product details: " + err.Error())
	}
	defer cursor.Close(ctx)

	var results []ProductDetail
	if err := cursor.All(ctx, &results); err != nil {
		app.Logger.Error("Failed to decode product details: " + err.Error())
	}

	return results
}

func (app *Crawler) GetDataCount(collection string) string {
	dataCollection := app.getCollection(collection)
	count, err := dataCollection.CountDocuments(context.TODO(), bson.D{{}})
	if err != nil {
		count = 0
	}

	return strconv.Itoa(int(count))
}
func (app *Crawler) GetErrorDataCount(collection string) int {
	dataCollection := app.getCollection(collection)

	// Add conditions for status: false and error: true
	filter := bson.D{
		{"status", false},
		{"error", true},
	}

	count, err := dataCollection.CountDocuments(context.TODO(), filter)
	if err != nil {
		count = 0
	}

	return int(count)
}

func (app *Crawler) GetErrorData(collection string) []UrlCollection {
	filterCondition := bson.D{
		{"status", false},
		{"error", true},
	}
	return app.filterUrlData(filterCondition, app.getCollection(collection))
}
