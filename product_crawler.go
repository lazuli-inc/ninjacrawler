package ninjacrawler

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
)

// CrawlPageDetail initiates the crawling process for detailed page information from the specified collection.
// It distributes the work among multiple goroutines and uses proxies if available.
func (app *Crawler) CrawlPageDetail(processorConfigs []ProcessorConfig) {
	for _, processorConfig := range processorConfigs {
		overrideEngineDefaults(app.engine, &processorConfig.Engine)
		app.toggleClient()
		processedUrls := make(map[string]bool) // Track processed URLs
		total := int32(0)
		app.crawlPageDetailRecursive(processorConfig, processedUrls, &total, 0)
		app.Logger.Info("Total %v %v Inserted ", atomic.LoadInt32(&total), processorConfig.OriginCollection)
		exportProductDetailsToCSV(app, processorConfig.Entity, 1)
	}
}

func (app *Crawler) crawlPageDetailRecursive(processorConfig ProcessorConfig, processedUrls map[string]bool, total *int32, counter int32) {
	urlCollections := app.getUrlCollections(processorConfig.OriginCollection)
	newUrlCollections := []UrlCollection{}
	for i, urlCollection := range urlCollections {
		if app.isLocalEnv && i >= app.engine.DevCrawlLimit {
			break
		}
		if !processedUrls[urlCollection.Url] {
			newUrlCollections = append(newUrlCollections, urlCollection)
		}
	}
	var wg sync.WaitGroup
	urlChan := make(chan UrlCollection, len(newUrlCollections))
	resultChan := make(chan interface{}, len(newUrlCollections))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, urlCollection := range newUrlCollections {
		urlChan <- urlCollection
		processedUrls[urlCollection.Url] = true // Mark URL as processed
	}
	close(urlChan)

	proxyCount := len(app.engine.ProxyServers)
	batchSize := app.engine.ConcurrentLimit
	totalUrls := len(newUrlCollections)
	goroutineCount := min(max(proxyCount, 1)*batchSize, totalUrls) // Determine the required number of goroutines

	for i := 0; i < goroutineCount; i++ {
		proxy := Proxy{}
		if proxyCount > 0 {
			proxy = app.engine.ProxyServers[i%proxyCount]
		}
		wg.Add(1)
		go func(proxy Proxy) {
			defer wg.Done()
			app.crawlWorker(ctx, processorConfig, urlChan, resultChan, proxy, app.isLocalEnv, &counter)
		}(proxy)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for results := range resultChan {
		switch v := results.(type) {
		case CrawlResult:
			switch res := v.Results.(type) {
			case *ProductDetail:
				invalidFields, unknownFields := validateRequiredFields(res, processorConfig.Preference.ValidationRules)
				if len(unknownFields) > 0 {
					app.Logger.Error("Unknown fields provided: %v", unknownFields)
					continue
				}
				if len(invalidFields) > 0 {
					html, _ := v.Document.Html()
					if app.engine.IsDynamic {
						html = app.getHtmlFromPage(v.Page)
					}
					app.Logger.Html(html, v.UrlCollection.Url, fmt.Sprintf("Validation failed from URL: %v. Missing value for required fields: %v", v.UrlCollection.Url, invalidFields))
					err := app.markAsError(v.UrlCollection.Url, processorConfig.OriginCollection)
					if err != nil {
						app.Logger.Info(err.Error())
						return
					}
					continue
				}

				app.saveProductDetail(processorConfig.Entity, res)
				if !app.isLocalEnv {
					err := app.submitProductData(res)
					if err != nil {
						app.Logger.Fatal("Failed to submit product data to API Server: %v", err)
						err := app.markAsError(v.UrlCollection.Url, processorConfig.OriginCollection)
						if err != nil {
							app.Logger.Info(err.Error())
							return
						}
					}
				}

				if !processorConfig.Preference.DoNotMarkAsComplete {
					err := app.markAsComplete(v.UrlCollection.Url, processorConfig.OriginCollection)
					if err != nil {
						app.Logger.Error(err.Error())
						continue
					}
				}
				atomic.AddInt32(total, 1)
			}
		}
	}
	if app.isLocalEnv && atomic.LoadInt32(&counter) >= int32(app.engine.DevCrawlLimit) {
		cancel()
		return
	}
	if app.isLocalEnv && atomic.LoadInt32(&counter) < int32(app.engine.DevCrawlLimit) {
		return
	}
	if len(newUrlCollections) > 0 {
		app.crawlPageDetailRecursive(processorConfig, processedUrls, total, counter)
	}
}

// validateRequiredFields checks if the required fields are non-empty in the ProductDetail struct.
// Returns two slices: one for invalid fields and one for unknown fields.
func validateRequiredFields(product *ProductDetail, requiredFields []string) ([]string, []string) {
	var invalidFields []string
	var unknownFields []string

	v := reflect.ValueOf(*product)
	t := v.Type()

	for _, field := range requiredFields {
		f, ok := t.FieldByName(field)
		if !ok {
			unknownFields = append(unknownFields, field)
			continue
		}
		fieldValue := v.FieldByName(field)
		if fieldValue.Kind() == reflect.String && fieldValue.String() == "" {
			invalidFields = append(invalidFields, f.Name)
		}
	}
	return invalidFields, unknownFields
}
