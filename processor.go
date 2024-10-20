package ninjacrawler

import (
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var locker sync.Mutex // Mutex to lock proxy access
var shouldRotateProxy = false

func (app *Crawler) Crawl(configs []ProcessorConfig) {
	for _, config := range configs {
		app.Logger.Summary("Starting: %s Crawler", config.OriginCollection)
		app.overrideEngineDefaults(app.engine, &config.Engine)
		app.toggleClient()

		var total int32 = 0
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
		// Assign proxy based on the current batch and strategy
		proxy := app.getProxy(int(atomic.LoadInt32(&batchCount)), proxies, &proxyLock)
		app.openBrowsers(proxy)

		// Loop through the URLs in the current batch
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

				proxyLock.Lock()
				app.applySleep()
				// Increment request count safely and apply sleep if needed
				atomic.AddInt32(&app.ReqCount, 1)
				app.CurrentCollection = config.OriginCollection
				app.CurrentUrlCollection = urlCollection
				//app.assignProxy(proxy)
				app.openPages()
				proxyLock.Unlock()
				ok := app.crawlWithProxies(urlCollection, config, 0, proxy)
				if ok && crawlLimit > 0 && atomic.AddInt32(total, 1) > int32(crawlLimit) {
					atomic.AddInt32(total, -1)
					shouldContinue = false
				}
			}(url, proxy)
		}

		wg.Wait()
		app.closeBrowsers()
		//proxyLock.Lock()
		atomic.AddInt32(&batchCount, 1)
		//proxyLock.Unlock()
		/*
			For test the rotate proxy uncomment this block
		*/
		//if atomic.LoadInt32(&batchCount)%2 == 0 {
		//	shouldRotateProxy = true
		//}
	}

	return shouldContinue
}
func (app *Crawler) getProxy(batchCount int, proxies []Proxy, proxyLock *sync.Mutex) Proxy {
	proxyLock.Lock()
	defer proxyLock.Unlock()

	if len(proxies) == 0 {
		return Proxy{}
	}
	proxyIndex := 0
	var proxy Proxy
	if app.engine.ProxyStrategy == ProxyStrategyConcurrency {
		proxyIndex = batchCount % len(proxies)
		proxy = proxies[proxyIndex]
	} else if app.engine.ProxyStrategy == ProxyStrategyRotation {
		proxyIndex = int(atomic.LoadInt32(&app.lastWorkingProxyIndex))
		if shouldRotateProxy {
			proxyIndex = (proxyIndex + 1) % len(proxies)
			app.Logger.Summary("Error with proxy %s: Retrying with proxy: %s", app.engine.ProxyServers[atomic.LoadInt32(&app.lastWorkingProxyIndex)].Server, app.engine.ProxyServers[proxyIndex].Server)
			shouldRotateProxy = false
		}
		proxy = proxies[proxyIndex]
	}
	atomic.StoreInt32(&app.CurrentProxyIndex, int32(proxyIndex))
	atomic.StoreInt32(&app.lastWorkingProxyIndex, int32(proxyIndex))
	return proxy
}
func (app *Crawler) applySleep() {
	// Random sleep between 500ms and 1s
	sleepDuration := time.Duration(rand.Intn(501)+500) * time.Millisecond
	time.Sleep(sleepDuration)
	if atomic.LoadInt32(&app.ReqCount) > 0 && atomic.LoadInt32(&app.ReqCount)%int32(app.engine.SleepAfter) == 0 {
		app.Logger.Info("Sleeping %d seconds after %d operations", app.engine.SleepDuration, app.engine.SleepAfter)
		time.Sleep(time.Duration(app.engine.SleepDuration) * time.Second)
	}
}

func (app *Crawler) assignProxy(proxy Proxy) {
	if len(app.engine.ProxyServers) == 0 && app.engine.ProxyStrategy == ProxyStrategyRotation {
		app.Logger.Fatal("No proxies available")
		return
	}
	if len(app.engine.ProxyServers) > 0 {
		app.CurrentProxy = proxy
		proxyIndex := atomic.LoadInt32(&app.CurrentProxyIndex) % int32(len(app.engine.ProxyServers))
		atomic.StoreInt32(&app.CurrentProxyIndex, proxyIndex)
	}
}

func (app *Crawler) crawlWithProxies(urlCollection UrlCollection, config ProcessorConfig, attempt int, proxy Proxy) bool {
	//fmt.Println("CurrentProxyIndex", atomic.LoadInt32(&app.CurrentProxyIndex))
	if app.runPreHandlers(config, urlCollection) {
		ctx, err := app.handleCrawlWorker(config, urlCollection, proxy)
		if err != nil {
			return app.handleCrawlError(err, urlCollection, config, attempt)
		}
		errExtract := app.extract(config, *ctx)
		if errExtract != nil {
			if strings.Contains(errExtract.Error(), "isRetryable") {
				return app.rotateProxy(errExtract, attempt)
			}
			app.Logger.Error(errExtract.Error())
			return false
		}
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
		return app.rotateProxy(err, attempt)
	}

	if markErr := app.MarkAsError(urlCollection.Url, config.OriginCollection, err.Error()); markErr != nil {
		app.Logger.Error("markErr: ", markErr.Error())
	}
	app.Logger.Error("Error crawling %s: %v", urlCollection.Url, err)
	return false
}

func (app *Crawler) rotateProxy(err error, attempt int) bool {
	if len(app.engine.ProxyServers) == 0 || app.engine.ProxyStrategy != ProxyStrategyRotation {
		return false
	}

	app.Logger.Warn("Retrying after %d requests with different proxy %s", atomic.LoadInt32(&app.ReqCount), err.Error())
	shouldRotateProxy = true

	if app.engine.RetrySleepDuration > 0 {
		app.Logger.Info("Sleeping %d seconds before retrying", app.engine.RetrySleepDuration)
		time.Sleep(time.Duration(app.engine.RetrySleepDuration) * time.Second)
	}

	if attempt >= len(app.engine.ProxyServers) {
		app.Logger.Info("All proxies exhausted.")
		return true
	}

	return false
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
