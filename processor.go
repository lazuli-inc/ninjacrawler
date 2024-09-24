package ninjacrawler

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func (app *Crawler) Crawl(configs []ProcessorConfig) {
	for _, config := range configs {
		app.Logger.Summary("Starting: %s Crawler", config.OriginCollection)
		app.overrideEngineDefaults(app.engine, &config.Engine)
		app.toggleClient()
		total := int32(0)
		for {
			productList := app.getUrlCollections(config.OriginCollection)
			if len(productList) == 0 {
				break
			}

			shouldContinue := app.processUrlsWithProxies(productList, config, &total)

			dataCount := total
			app.Logger.Summary("[Total (%d) :%s: found from :%s:]", dataCount, config.Entity, config.OriginCollection)

			if errCount := app.GetErrorDataCount(config.OriginCollection); errCount > 0 {
				app.Logger.Summary("Error count: %d", errCount)
			}
			if !shouldContinue {
				app.Logger.Warn("Dev crawl limit of %d reached, stopping...", app.engine.DevCrawlLimit)
				break
			}
		}

		// Consolidate similar cases in switch statement
		switch config.Processor.(type) {
		case ProductDetailSelector, ProductDetailApi, func(CrawlerContext, func([]ProductDetailSelector, string)) error:
			dataCount := app.GetDataCount(config.Entity)
			app.Logger.Summary("Data count: %s", dataCount)
			exportProductDetailsToCSV(app, config.Entity, 1)
		}
	}
}

func (app *Crawler) processUrlsWithProxies(urls []UrlCollection, config ProcessorConfig, total *int32) bool {
	var wg sync.WaitGroup
	proxies := app.engine.ProxyServers
	shouldContinue := true
	reqCount := int32(0)
	totalReqCount := 0
	batchCount := 0

	for batchIndex := 0; batchIndex < len(urls); batchIndex += app.engine.ConcurrentLimit {
		batchCount++

		if !shouldContinue {
			break
		}
		for i := batchIndex; i < batchIndex+app.engine.ConcurrentLimit && i < len(urls); i++ {
			// Use atomic load to check total in a thread-safe way
			if app.engine.DevCrawlLimit > 0 && atomic.LoadInt32(total) >= int32(app.engine.DevCrawlLimit) {
				shouldContinue = false
				break
			}

			// Determine the current proxy to use for the entire batch
			proxy := Proxy{}
			if len(proxies) > 0 && app.engine.ProxyStrategy == ProxyStrategyConcurrency {
				// Use a new proxy for each request
				proxy = proxies[totalReqCount%len(proxies)]
			} else if len(proxies) > 0 && app.engine.ProxyStrategy == ProxyStrategyRotation {
				proxy = proxies[0]
			}

			totalReqCount++
			url := urls[i]
			wg.Add(1)

			go func(urlCollection UrlCollection, proxy Proxy) {
				defer func() {
					if r := recover(); r != nil {
						app.HandlePanic(r)
					}
					wg.Done()
				}()
				// Atomically check and increment the total count
				if app.engine.DevCrawlLimit > 0 && atomic.AddInt32(total, 1) > int32(app.engine.DevCrawlLimit) {
					atomic.AddInt32(total, -1)
					shouldContinue = false
					return
				}
				atomic.AddInt32(&reqCount, 1)
				// Freeze all goroutines every n requests
				if reqCount > 0 && atomic.LoadInt32(&reqCount)%int32(app.engine.SleepAfter) == 0 {
					app.Logger.Info("Sleeping %d seconds after %d operations", app.engine.SleepDuration, app.engine.SleepAfter)
					time.Sleep(time.Duration(app.engine.SleepDuration) * time.Second)
				}
				app.CurrentCollection = config.OriginCollection
				app.CurrentUrlCollection = urlCollection
				preHandlerError := false
				if config.Preference.PreHandlers != nil { // Execute pre handlers
					for _, preHandler := range config.Preference.PreHandlers {
						err := preHandler(PreHandlerContext{UrlCollection: urlCollection, App: app})
						if err != nil {
							preHandlerError = true
						}
					}
				}
				if !preHandlerError {
					ctx, err := app.handleCrawlWorker(config, proxy, urlCollection)
					if err != nil {
						if strings.Contains(err.Error(), "StatusCode:404") {
							if markMaxErr := app.MarkAsMaxErrorAttempt(urlCollection.Url, config.OriginCollection, err.Error()); markMaxErr != nil {
								app.Logger.Error("markMaxErr: ", markMaxErr.Error())
								return
							}
						} else if strings.Contains(err.Error(), "isRetryable") {
							if len(proxies) > 0 && app.engine.ProxyStrategy == ProxyStrategyRotation {
								proxyIndex := (batchCount) % len(proxies)
								proxy = proxies[proxyIndex]
								app.Logger.Warn("Rotating proxy to %s", proxy.Server)
							}
							if markErr := app.MarkAsError(urlCollection.Url, config.OriginCollection, err.Error()); markErr != nil {
								app.Logger.Error("markErr: ", markErr.Error())
								return
							}
						} else {
							if markErr := app.MarkAsError(urlCollection.Url, config.OriginCollection, err.Error()); markErr != nil {
								app.Logger.Error("markErr: ", markErr.Error())
								return
							}
						}
						app.Logger.Error("Error crawling %s: %v", urlCollection.Url, err)
						return
					}
					app.extract(config, *ctx)
				}

			}(url, proxy)

			//*total++
			//atomic.AddInt32(total, 1)
		}

		// Wait for all goroutines within a batch to finish
		wg.Wait()
	}

	return shouldContinue
}
