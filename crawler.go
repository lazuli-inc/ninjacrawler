package ninjacrawler

import (
	"context"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/playwright-community/playwright-go"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

func (app *Crawler) crawlWorker(ctx context.Context, processorConfig ProcessorConfig, urlChan <-chan UrlCollection, resultChan chan<- interface{}, proxy Proxy, isLocalEnv bool, counter *int32) {
	var page playwright.Page
	var browser playwright.Browser
	var err error
	var doc *goquery.Document
	var apiResponse map[string]interface{}

	if app.engine.IsDynamic {
		browser, page, err = app.GetBrowserPage(app.pw, app.engine.BrowserType, proxy)
		if err != nil {
			app.Logger.Fatal("failed to initialize browser with Proxy: %v\n", err)
		}
		defer browser.Close()
		defer page.Close()
	}

	operationCount := 0 // Initialize operation count

	for {
		select {
		case <-ctx.Done():
			return
		case urlCollection, more := <-urlChan:
			if !more {
				return
			}
			crawlableUrl := urlCollection.Url
			if urlCollection.CurrentPageUrl != "" {
				crawlableUrl = urlCollection.CurrentPageUrl
			}

			if proxy.Server != "" {
				app.Logger.Info("Crawling :%s: %s using Proxy %s", processorConfig.OriginCollection, crawlableUrl, proxy.Server)
			} else {
				app.Logger.Info("Crawling :%s: %s", processorConfig.OriginCollection, crawlableUrl)
			}
			if app.engine.IsDynamic {
				doc, err = app.NavigateToURL(page, crawlableUrl)
			} else {
				switch processorConfig.Processor.(type) {
				case ProductDetailApi:
					apiResponse, err = app.NavigateToApiURL(app.httpClient, crawlableUrl, proxy)
				default:
					doc, err = app.NavigateToStaticURL(app.httpClient, crawlableUrl, proxy)
				}
			}

			if err != nil {
				markAsError := app.markAsError(urlCollection.Url, processorConfig.OriginCollection)
				if markAsError != nil {
					app.Logger.Error(markAsError.Error())
					return
				}
				app.Logger.Error(err.Error())
				continue
			}

			crawlerCtx := CrawlerContext{
				App:           app,
				Document:      doc,
				UrlCollection: urlCollection,
				Page:          page,
				ApiResponse:   apiResponse,
			}

			var results interface{}
			switch v := processorConfig.Processor.(type) {
			case func(CrawlerContext) []UrlCollection:
				var collections []UrlCollection
				collections = v(crawlerCtx)

				for _, item := range collections {
					if item.Parent == "" && processorConfig.OriginCollection != baseCollection {
						app.Logger.Fatal("Missing Parent Url, Invalid OriginCollection: %v", item)
						continue
					}
				}
				app.insert(processorConfig.Entity, collections, urlCollection.Url)
				if !processorConfig.Preference.DoNotMarkAsComplete {
					err := app.markAsComplete(urlCollection.Url, processorConfig.OriginCollection)
					if err != nil {
						app.Logger.Error(err.Error())
						continue
					}
				}
				atomic.AddInt32(counter, 1)
			case func(CrawlerContext, func([]UrlCollection, string)) error:
				handleErr := v(crawlerCtx, func(collections []UrlCollection, currentPageUrl string) {
					for _, item := range collections {
						if item.Parent == "" && processorConfig.OriginCollection != baseCollection {
							app.Logger.Fatal("Missing Parent Url, Invalid OriginCollection: %v", item)
							continue
						}
					}
					if currentPageUrl != "" && currentPageUrl != urlCollection.Url {
						currentPageErr := app.SyncCurrentPageUrl(urlCollection.Url, currentPageUrl, processorConfig.OriginCollection)
						if currentPageErr != nil {
							app.Logger.Fatal(currentPageErr.Error())
							return
						}
					}
					app.insert(processorConfig.Entity, collections, urlCollection.Url)
					atomic.AddInt32(counter, 1)
				})
				if handleErr != nil {
					markAsError := app.markAsError(urlCollection.Url, processorConfig.OriginCollection)
					if markAsError != nil {
						app.Logger.Info(markAsError.Error())
						return
					}
					app.Logger.Error(handleErr.Error())
				} else {
					if !processorConfig.Preference.DoNotMarkAsComplete {
						err := app.markAsComplete(urlCollection.Url, processorConfig.OriginCollection)
						if err != nil {
							app.Logger.Error(err.Error())
							continue
						}
					}
				}

			case UrlSelector:
				var collections []UrlCollection
				collections = app.processDocument(doc, v, urlCollection)

				for _, item := range collections {
					if item.Parent == "" && processorConfig.OriginCollection != baseCollection {
						app.Logger.Fatal("Missing Parent Url, Invalid OriginCollection: %v", item)
						continue
					}
				}
				app.insert(processorConfig.Entity, collections, urlCollection.Url)

				if !processorConfig.Preference.DoNotMarkAsComplete {
					err := app.markAsComplete(urlCollection.Url, processorConfig.OriginCollection)
					if err != nil {
						app.Logger.Error(err.Error())
						continue
					}
				}
				atomic.AddInt32(counter, 1)

			case ProductDetailSelector:
				results = crawlerCtx.handleProductDetail(processorConfig.Processor)
			case ProductDetailApi:
				results = crawlerCtx.handleProductDetailApi(processorConfig.Processor)

			default:
				app.Logger.Fatal("Unsupported processor type: %T", processorConfig.Processor)
			}

			crawlResult := CrawlResult{
				Results:       results,
				UrlCollection: urlCollection,
				Page:          page,
				Document:      doc,
			}

			select {
			case resultChan <- crawlResult:
				atomic.AddInt32(counter, 1)
			default:
				app.Logger.Info("Channel is full, dropping Item")
			}
			if isLocalEnv && atomic.LoadInt32(counter) >= int32(app.engine.DevCrawlLimit) {
				app.Logger.Warn("Dev Crawl limit reached!")
				return
			}

			operationCount++                               // Increment the operation count
			if operationCount%app.engine.SleepAfter == 0 { // Check if 10 operations have been performed
				app.Logger.Info("Sleeping 10 Second after %d Operations", app.engine.SleepAfter)
				time.Sleep(10 * time.Second) // Sleep for 10 seconds
			}
		}
	}
}

func (app *Crawler) CrawlUrls(processorConfigs []ProcessorConfig) {
	for _, processorConfig := range processorConfigs {
		overrideEngineDefaults(app.engine, &processorConfig.Engine)
		app.toggleClient()
		processedUrls := make(map[string]bool) // Track processed URLs
		total := int32(0)
		app.crawlUrlsRecursive(processorConfig, processedUrls, &total, 0)
		if atomic.LoadInt32(&total) > 0 {
			app.Logger.Info("[Total (%d) :%s: found from :%s:]", atomic.LoadInt32(&total), processorConfig.Entity, processorConfig.OriginCollection)
		}
	}
}
func (app *Crawler) crawlUrlsRecursive(processorConfig ProcessorConfig, processedUrls map[string]bool, total *int32, counter int32) {
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
	goroutineCount := min(max(proxyCount, 1)*batchSize, totalUrls)

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
			case []UrlCollection:
				atomic.AddInt32(total, int32(len(res)))
				app.Logger.Info("(%d) :%s: Found From [%s => %s]", len(res), processorConfig.Entity, processorConfig.OriginCollection, v.UrlCollection.Url)
			}
		}
	}

	if app.isLocalEnv && atomic.LoadInt32(&counter) >= int32(app.engine.DevCrawlLimit) {
		cancel()
		return
	}
	if len(newUrlCollections) > 0 && (app.isLocalEnv && atomic.LoadInt32(&counter) < int32(app.engine.DevCrawlLimit)) {
		app.crawlUrlsRecursive(processorConfig, processedUrls, total, counter)
	}
}

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
	if len(newUrlCollections) > 0 && (app.isLocalEnv && atomic.LoadInt32(&counter) < int32(app.engine.DevCrawlLimit)) {
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
