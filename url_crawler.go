package ninjacrawler

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Deprecated: CrawlUrls is deprecated and will be removed in a future version.
// Use Crawl instead, which includes improvements for proxy rotation and error handling.
func (app *Crawler) CrawlUrls(processorConfigs []ProcessorConfig) {
	for _, processorConfig := range processorConfigs {
		app.Logger.Summary("Starting :%s: Crawler", processorConfig.OriginCollection)
		app.overrideEngineDefaults(app.engine, &processorConfig.Engine)
		app.toggleClient()
		processedUrls := make(map[string]bool) // Track processed URLs
		total := int32(0)
		app.crawlUrlsRecursive(processorConfig, processedUrls, &total, 0)
		if atomic.LoadInt32(&total) > 0 {
			app.Logger.Info("[Total (%d) :%s: found from :%s:]", atomic.LoadInt32(&total), processorConfig.Entity, processorConfig.OriginCollection)
		}
		dataCount := app.GetDataCount(processorConfig.Entity)
		app.Logger.Summary("[Total (%s) :%s: found from :%s:]", dataCount, processorConfig.Entity, processorConfig.OriginCollection)

		errDataCount := app.GetErrorDataCount(processorConfig.OriginCollection)
		if errDataCount > 0 {
			app.Logger.Summary("Error count: %s", errDataCount)
		}
	}
}
func (app *Crawler) crawlUrlsRecursive(processorConfig ProcessorConfig, processedUrls map[string]bool, total *int32, counter int32) {
	for {
		productListData := app.getUrlCollections(processorConfig.OriginCollection)

		if len(productListData) == 0 {
			return // Exit recursion if no data to process
		}

		var wg sync.WaitGroup

		// Process URLs in the current level
		app.processUrls(productListData, processorConfig, processedUrls, total, counter, &wg)

		// Wait for all goroutines to finish
		wg.Wait()
	}
}

func (app *Crawler) processUrls(productListData []UrlCollection, processorConfig ProcessorConfig, processedUrls map[string]bool, total *int32, counter int32, wg *sync.WaitGroup) {
	// Add the processing of the current level
	wg.Add(1)
	go app.handleJob(productListData, processorConfig, processedUrls, total, counter, wg)
}

func (app *Crawler) handleJob(urlCollections []UrlCollection, processorConfig ProcessorConfig, processedUrls map[string]bool, total *int32, counter int32, wg *sync.WaitGroup) {
	defer wg.Done()

	// Set a timeout for the context to prevent infinite waiting.
	timeoutDuration := time.Duration(app.engine.CrawlTimeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	defer cancel()

	urlChan := make(chan UrlCollection, len(urlCollections))
	resultChan := make(chan interface{}, len(urlCollections))

	// Initialize the URL channel with the provided URLs
	for _, urlCollection := range urlCollections {
		urlChan <- urlCollection
		if urlCollection.Attempts > 0 && urlCollection.Attempts <= app.engine.MaxRetryAttempts {
			processedUrls[urlCollection.CurrentPageUrl] = false // Do not mark URL as processed
			processedUrls[urlCollection.Url] = false            // Do not mark URL as processed
		} else {
			processedUrls[urlCollection.CurrentPageUrl] = true // Mark URL as processed
			processedUrls[urlCollection.Url] = true            // Mark URL as processed
		}
	}
	close(urlChan)

	proxyCount := len(app.engine.ProxyServers)
	batchSize := app.engine.ConcurrentLimit
	totalUrls := len(urlCollections)
	goroutineCount := min(max(proxyCount, 1)*batchSize, totalUrls)
	if app.engine.ProxyStrategy == ProxyStrategyRotation {
		goroutineCount = min(batchSize, totalUrls)
	}

	var innerWg sync.WaitGroup

	for i := 0; i < goroutineCount; i++ {
		proxy := Proxy{}
		currentProxyIndex := int32(0)
		if proxyCount > 0 {
			proxy = app.engine.ProxyServers[i%proxyCount]
			currentProxyIndex = int32(i % proxyCount)
		}
		innerWg.Add(1)
		go func(proxy Proxy) {
			defer innerWg.Done()
			defer func() {
				if r := recover(); r != nil {
					app.HandlePanic(r)
				}
			}()
			app.CurrentCollection = processorConfig.OriginCollection
			app.crawlWorker(ctx, processorConfig, urlChan, resultChan, app.isLocalEnv, &counter, &currentProxyIndex)
		}(proxy)
	}

	// Close the result channel after all workers have finished
	go func() {
		innerWg.Wait()
		close(resultChan)
	}()

	// Process the results as they come in
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

	// Cancel the context if the dev crawl limit is reached
	if app.isLocalEnv && atomic.LoadInt32(&counter) >= int32(app.engine.DevCrawlLimit) {
		app.Logger.Warn("Dev Crawl limit reached. Cancelling job...")
		cancel()
	}
}
