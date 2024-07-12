package ninjacrawler

import (
	"context"
	"sync"
	"sync/atomic"
)

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
		if !processedUrls[urlCollection.CurrentPageUrl] || !processedUrls[urlCollection.Url] {
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
		if urlCollection.Attempts > 0 && urlCollection.Attempts <= app.engine.MaxRetryAttempts {
			processedUrls[urlCollection.CurrentPageUrl] = false // Do Not Mark URL as processed
			processedUrls[urlCollection.Url] = false            // Do Not Mark URL as processed
		} else {
			processedUrls[urlCollection.CurrentPageUrl] = true // Mark URL as processed
			processedUrls[urlCollection.Url] = true            // Mark URL as processed
		}
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
	if len(newUrlCollections) > 0 {
		app.crawlUrlsRecursive(processorConfig, processedUrls, total, counter)
	}
}
