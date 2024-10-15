package ninjacrawler

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var locker sync.Mutex // Mutex to lock proxy access
func (app *Crawler) Crawl(configs []ProcessorConfig) {
	for _, config := range configs {
		app.Logger.Summary("Starting: %s Crawler", config.OriginCollection)
		app.overrideEngineDefaults(app.engine, &config.Engine)
		app.toggleClient()
		total := int32(0)
		dataCount, _ := strconv.Atoi(app.GetDataCount(config.Entity))
		if dataCount > 0 {
			total = int32(dataCount)
		}
		crawlLimit := app.getCrawlLimit()

		for {
			productList := app.getUrlCollections(config.OriginCollection)
			if len(productList) == 0 {
				break
			}

			shouldContinue := app.processUrlsWithProxies(productList, config, &total, crawlLimit)

			if !shouldContinue {
				app.Logger.Debug("Crawl limit of %d reached, stopping...", crawlLimit)
				break
			}
		}

		app.logSummary(config)
		app.processPostCrawl(config)
	}
}

func (app *Crawler) getCrawlLimit() int {
	if app.isLocalEnv && app.engine.DevCrawlLimit > 0 {
		return app.engine.DevCrawlLimit
	} else if app.isStgEnv && app.engine.StgCrawlLimit > 0 {
		return app.engine.StgCrawlLimit
	}
	return 0
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

		proxy := app.getProxy(int(atomic.LoadInt32(&batchCount)), proxies, &proxyLock)
		app.openBrowsers(proxy)

		for i := batchIndex; i < batchIndex+app.engine.ConcurrentLimit && i < len(urls); i++ {
			if crawlLimit > 0 && atomic.LoadInt32(total) >= int32(crawlLimit) {
				shouldContinue = false
				break
			}

			url := urls[i]
			wg.Add(1)

			go func(urlCollection UrlCollection, proxy Proxy) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						app.HandlePanic(r)
					}
				}()

				atomic.AddInt32(&app.ReqCount, 1)
				app.applySleep()

				app.CurrentCollection = config.OriginCollection
				app.CurrentUrlCollection = urlCollection
				app.assignProxy(proxy, &proxyLock)

				proxyLock.Lock()
				app.openPages()
				proxyLock.Unlock()
				ok := app.crawlWithProxies(urlCollection, config, 0)
				if ok && crawlLimit > 0 && atomic.AddInt32(total, 1) > int32(crawlLimit) {
					atomic.AddInt32(total, -1)
					shouldContinue = false
				}
			}(url, proxy)
		}

		wg.Wait()
		app.closeBrowsers()
		proxyLock.Lock()
		atomic.AddInt32(&batchCount, 1)
		proxyLock.Unlock()
	}

	return shouldContinue
}
func (app *Crawler) getProxy(batchCount int, proxies []Proxy, proxyLock *sync.Mutex) Proxy {
	proxyLock.Lock()
	defer proxyLock.Unlock()

	if len(proxies) == 0 {
		return Proxy{}
	}

	var proxy Proxy
	if app.engine.ProxyStrategy == ProxyStrategyConcurrency {
		proxy = proxies[batchCount%len(proxies)]
	} else if app.engine.ProxyStrategy == ProxyStrategyRotation {
		proxyIndex := int(atomic.LoadInt32(&app.lastWorkingProxyIndex))
		proxy = proxies[proxyIndex]
	}
	return proxy
}
func (app *Crawler) applySleep() {
	if app.ReqCount > 0 && atomic.LoadInt32(&app.ReqCount)%int32(app.engine.SleepAfter) == 0 {
		app.Logger.Info("Sleeping %d seconds after %d operations", app.engine.SleepDuration, app.engine.SleepAfter)
		time.Sleep(time.Duration(app.engine.SleepDuration) * time.Second)
	}
}

func (app *Crawler) assignProxy(proxy Proxy, proxyLock *sync.Mutex) {
	if len(app.engine.ProxyServers) == 0 && app.engine.ProxyStrategy == ProxyStrategyRotation {
		app.Logger.Fatal("No proxies available")
		return
	}
	if len(app.engine.ProxyServers) > 0 {
		proxyLock.Lock()
		app.CurrentProxy = proxy
		atomic.StoreInt32(&app.CurrentProxyIndex, atomic.LoadInt32(&app.CurrentProxyIndex)%int32(len(app.engine.ProxyServers)))

		proxyLock.Unlock()
	}
}

func (app *Crawler) crawlWithProxies(urlCollection UrlCollection, config ProcessorConfig, attempt int) bool {
	if app.runPreHandlers(config, urlCollection) {
		ctx, err := app.handleCrawlWorker(config, urlCollection)
		if err != nil {
			return app.handleCrawlError(err, urlCollection, config, attempt)
		}
		app.processCrawlSuccess(ctx, config)
		return true
	}
	return false
}

func (app *Crawler) runPreHandlers(config ProcessorConfig, urlCollection UrlCollection) bool {
	if config.Preference.PreHandlers != nil {
		for _, preHandler := range config.Preference.PreHandlers {
			if err := preHandler(PreHandlerContext{UrlCollection: urlCollection, App: app}); err != nil {
				return false
			}
		}
	}
	return true
}

func (app *Crawler) handleCrawlError(err error, urlCollection UrlCollection, config ProcessorConfig, attempt int) bool {
	if strings.Contains(err.Error(), "StatusCode:404") {
		if markMaxErr := app.MarkAsMaxErrorAttempt(urlCollection.Url, config.OriginCollection, err.Error()); markMaxErr != nil {
			app.Logger.Error("markMaxErr: ", markMaxErr.Error())
			return false
		}
	}

	if strings.Contains(err.Error(), "isRetryable") {
		return app.retryWithDifferentProxy(err, urlCollection, config, attempt)
	}

	if markErr := app.MarkAsError(urlCollection.Url, config.OriginCollection, err.Error()); markErr != nil {
		app.Logger.Error("markErr: ", markErr.Error())
	}
	app.Logger.Error("Error crawling %s: %v", urlCollection.Url, err)
	return false
}

func (app *Crawler) retryWithDifferentProxy(err error, urlCollection UrlCollection, config ProcessorConfig, attempt int) bool {
	if len(app.engine.ProxyServers) == 0 || app.engine.ProxyStrategy != ProxyStrategyRotation {
		return false
	}
	nextProxyIndex := (atomic.LoadInt32(&app.lastWorkingProxyIndex) + 1) % int32(len(app.engine.ProxyServers))
	app.Logger.Summary("Error with proxy %s: %v. Retrying with proxy: %s", app.engine.ProxyServers[atomic.LoadInt32(&app.lastWorkingProxyIndex)].Server, err.Error(), app.engine.ProxyServers[nextProxyIndex].Server)

	atomic.StoreInt32(&app.CurrentProxyIndex, nextProxyIndex)
	app.CurrentProxy = app.engine.ProxyServers[nextProxyIndex]

	atomic.StoreInt32(&app.lastWorkingProxyIndex, nextProxyIndex)
	if app.engine.RetrySleepDuration > 0 {
		app.Logger.Info("Sleeping %d seconds before retrying", app.engine.RetrySleepDuration)
		time.Sleep(time.Duration(app.engine.RetrySleepDuration) * time.Second)
	}

	if attempt >= len(app.engine.ProxyServers) {
		app.Logger.Info("All proxies exhausted.")
		return true
	}

	return app.crawlWithProxies(urlCollection, config, attempt+1)
}

func (app *Crawler) processCrawlSuccess(ctx *CrawlerContext, config ProcessorConfig) {
	app.extract(config, *ctx)
	atomic.StoreInt32(&app.lastWorkingProxyIndex, app.CurrentProxyIndex)
}

func (app *Crawler) logSummary(config ProcessorConfig) {
	dataCount := app.GetDataCount(config.Entity)
	app.Logger.Summary("[Total (%s) :%s: found from :%s:]", dataCount, config.Entity, config.OriginCollection)

	if errCount := app.GetErrorDataCount(config.OriginCollection); errCount > 0 {
		app.Logger.Summary("Error count: %d", errCount)
	}
}

func (app *Crawler) processPostCrawl(config ProcessorConfig) {
	switch config.Processor.(type) {
	case ProductDetailSelector, ProductDetailApi, func(CrawlerContext, func([]ProductDetailSelector, string)) error:
		dataCount := app.GetDataCount(config.Entity)
		app.Logger.Summary("Data count: %s", dataCount)
		exportProductDetailsToCSV(app, config.Entity, 1)
	}
}

// defer close http client
