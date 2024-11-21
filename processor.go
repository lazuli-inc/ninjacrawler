package ninjacrawler

import (
	"fmt"
	"github.com/temoto/robotstxt"
	"math/rand"
	"net/url"
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

		app.CurrentProcessorConfig = config
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
		proxy := Proxy{}
		proxy = app.getProxy(int(atomic.LoadInt32(&batchCount)), proxies, &proxyLock)
		if !app.batchAbleStrategy() {
			app.openBrowsers(proxy)
		}

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
				if app.batchAbleStrategy() {
					if *app.engine.IsDynamic {
						app.Logger.Fatal("Dynamic mode is not supported with current strategy")
					}
					proxy = app.getProxy(int(atomic.LoadInt32(&batchCount)), proxies, &proxyLock)
					app.openBrowsers(proxy)
					defer app.closeBrowsers()
				}
				if app.engine.ApplyRandomSleep != nil && *app.engine.ApplyRandomSleep {
					app.applySleep()
				}
				// Increment request count safely and apply sleep if needed
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
			}(collection, proxy)

			if app.batchAbleStrategy() {
				atomic.AddInt32(&batchCount, 1)
			}

		}

		wg.Wait()
		//proxyLock.Lock()
		if !app.batchAbleStrategy() {
			app.closeBrowsers()
			atomic.AddInt32(&batchCount, 1)
		}

	}

	return shouldContinue
}
func (app *Crawler) batchAbleStrategy() bool {
	return app.engine.ProxyStrategy == ProxyStrategyRotationPerBatch
}
func (app *Crawler) getProxy(batchCount int, proxies []Proxy, proxyLock *sync.Mutex) Proxy {
	proxyLock.Lock()
	defer proxyLock.Unlock()
	if len(app.engine.ProxyServers) == 0 && app.engine.ProxyStrategy == ProxyStrategyRotation {
		app.Logger.Fatal("No proxies provided for rotation")
	}
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
	} else if app.engine.ProxyStrategy == ProxyStrategyRotationPerBatch {
		proxyIndex = int(atomic.LoadInt32(&app.lastWorkingProxyIndex))
		proxyIndex = (proxyIndex + 1) % len(proxies)
		shouldRotateProxy = false
		if batchCount == 0 {
			proxyIndex = 0
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

func (app *Crawler) crawlWithProxies(page interface{}, urlCollection UrlCollection, config ProcessorConfig, attempt int, proxy Proxy) bool {
	//fmt.Println("CurrentProxyIndex", atomic.LoadInt32(&app.CurrentProxyIndex))
	if app.runPreHandlers(config, urlCollection) {
		ctx, err := app.handleCrawlWorker(page, config, urlCollection, proxy)
		if err != nil {
			return app.handleCrawlError(err, urlCollection, config, attempt)
		}
		errExtract := app.extract(page, config, *ctx)
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

	if markErr := app.MarkAsError(urlCollection.Url, config.OriginCollection, err.Error()); markErr != nil {
		app.Logger.Error("markErr: ", markErr.Error())
	}
	app.Logger.Error("Error crawling %s: %v", urlCollection.Url, err)

	if strings.Contains(err.Error(), "isRetryable") {
		return app.rotateProxy(err, attempt)
	}
	return false
}

func (app *Crawler) rotateProxy(err error, attempt int) bool {
	if app.engine.RetrySleepDuration > 0 {
		app.Logger.Info("Sleeping %d minutes before retrying", app.engine.RetrySleepDuration)
		time.Sleep(time.Duration(app.engine.RetrySleepDuration) * time.Minute)
	}
	if len(app.engine.ProxyServers) == 0 || app.engine.ProxyStrategy != ProxyStrategyRotation {
		return false
	}

	app.Logger.Warn("Retrying after %d requests with different proxy %s", atomic.LoadInt32(&app.ReqCount), err.Error())
	shouldRotateProxy = true

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
func shouldCrawl(fullURL string, robotsData *robotstxt.RobotsData, userAgent string) bool {
	if robotsData == nil {
		return true
	}
	group := robotsData.FindGroup(userAgent)

	// Extract only the path part of the URL for testing
	parsedURL, err := url.Parse(fullURL)
	if err != nil {
		fmt.Println("[shouldCrawl] Error parsing fullURL:", err)
		return false
	}

	return group.Test(parsedURL.Path)
}
