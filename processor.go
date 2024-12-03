package ninjacrawler

import (
	"strconv"
	"sync"
	"sync/atomic"
)

func (app *Crawler) Crawl(configs []ProcessorConfig) {
	for _, config := range configs {
		app.Logger.Summary("Starting: %s Crawler", config.OriginCollection)
		app.overrideEngineDefaults(app.engine, &config.Engine)

		app.CurrentProcessorConfig = config
		var total int32 = 0
		dataCount, _ := strconv.Atoi(app.GetDataCount(config.Entity))
		if dataCount > 0 {
			total = int32(dataCount)
		}
		crawlLimit := app.getCrawlLimit()

		for {
			var productList []UrlCollection
			if config.Preference.ValidationRetryConfig != nil {
				retryableProductLists := app.getRetryableUrlCollections(config.OriginCollection)
				if len(retryableProductLists) > 0 {
					productList = retryableProductLists
					app.overrideEngineDefaults(app.engine, config.Preference.ValidationRetryConfig)
				} else {
					productList = app.getUrlCollections(config.OriginCollection)
				}
			} else {
				productList = app.getUrlCollections(config.OriginCollection)
			}
			if len(productList) == 0 {
				break
			}

			shouldContinue := false
			if app.engine.ProxyStrategy == ProxyStrategyRotationPerBatch {
				if *app.engine.IsDynamic {
					app.Logger.Fatal("Dynamic mode is not supported with current strategy")
				}
				shouldContinue = app.processStaticUrlsWithProxies(productList, config, &total, crawlLimit)
			} else {
				shouldContinue = app.processUrlsWithProxies(productList, config, &total, crawlLimit)
			}

			if !shouldContinue {
				app.Logger.Debug("Crawl limit of %d reached, stopping...", crawlLimit)
				break
			}
		}

		app.logSummary(config)
		app.processPostCrawl(config)
	}
}

func (app *Crawler) processUrlsWithProxies(urls []UrlCollection, config ProcessorConfig, total *int32, crawlLimit int) bool {
	var wg sync.WaitGroup
	proxies := app.engine.ProxyServers
	shouldContinue := true
	var batchCount int32 = 0
	var proxyLock sync.Mutex // Mutex to lock proxy access

	for batchIndex := 0; batchIndex < len(urls); batchIndex += app.engine.ConcurrentLimit {

		if !shouldContinue {
			break
		}
		proxy := Proxy{}
		proxy = app.getProxy(int(atomic.LoadInt32(&batchCount)), proxies, &proxyLock)

		app.openBrowsers(proxy)

		// Loop through the URLs in the current batch
		for i := batchIndex; i < batchIndex+app.engine.ConcurrentLimit && i < len(urls); i++ {
			if crawlLimit > 0 && atomic.LoadInt32(total) >= int32(crawlLimit) {
				shouldContinue = false
				break
			}

			collection := urls[i]

			if app.preference.CheckRobotsTxt != nil && *app.preference.CheckRobotsTxt {
				if !shouldCrawl(collection.Url, app.robotsData, app.GetUserAgent()) {
					app.Logger.Debug("[SKIP] Robots disallowed: %s", collection.Url)
					continue
				}
				// do check other stuff like CrawlDelay
			}

			wg.Add(1)

			go func(urlCollection UrlCollection, proxy Proxy) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						app.HandlePanic(r)
					}
				}()
				app.applySleep()
				atomic.AddInt32(&app.ReqCount, 1)
				app.CurrentCollection = config.OriginCollection
				app.CurrentUrlCollection = urlCollection
				//app.assignProxy(proxy)
				page := app.openPages()
				defer app.closePages(page)
				ok := app.crawlWithProxies(page, urlCollection, config, 0, proxy)
				if ok && crawlLimit > 0 && atomic.AddInt32(total, 1) > int32(crawlLimit) {
					atomic.AddInt32(total, -1)
					shouldContinue = false
				}
				app.syncRequestMetrics()
			}(collection, proxy)

		}

		wg.Wait()
		//proxyLock.Lock()

		app.closeBrowsers()
		atomic.AddInt32(&batchCount, 1)

	}

	return shouldContinue
}
