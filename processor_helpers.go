package ninjacrawler

import (
	"encoding/json"
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

func (app *Crawler) getCrawlLimit() int {
	if app.isLocalEnv && app.engine.DevCrawlLimit > 0 {
		return app.engine.DevCrawlLimit
	} else if app.isStgEnv && app.engine.StgCrawlLimit > 0 {
		return app.engine.StgCrawlLimit
	}
	return 0
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
		proxyIndex = int(atomic.LoadInt32(&app.lastWorkingProxyIndex))
		proxyIndex = (proxyIndex + 1) % len(proxies)
		shouldRotateProxy = false
		if batchCount == 0 {
			proxyIndex = 0
		}
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

	if app.engine.ApplyRandomSleep != nil && *app.engine.ApplyRandomSleep {
		sleepDuration := time.Duration(rand.Intn(501)+500) * time.Millisecond
		time.Sleep(sleepDuration)
	}
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
	type CrawlingError struct {
		Url          string `json:"url" bson:"url"`
		ErrorMessage string `json:"error_message" bson:"error_message"`
	}
	type CrawlingSummary struct {
		SiteID         string          `json:"site_id" bson:"site_id"`
		CollectionName string          `json:"collection_name" bson:"collection_name"`
		DataCount      int32           `json:"data_count" bson:"data_count"`
		ErrorCount     int32           `json:"error_count" bson:"error_count"`
		Errors         []CrawlingError `json:"errors" bson:"errors"`
		CreatedAt      time.Time       `json:"created_at" bson:"created_at"`
	}
	var crawlingErrors []CrawlingError
	dataCount := app.GetDataCount(config.Entity)
	app.Logger.Summary("[Total (%s) :%s: found from :%s:]", dataCount, config.Entity, config.OriginCollection)

	errData := app.GetErrorData(config.OriginCollection)
	if len(errData) > 0 {
		for _, data := range errData {
			crawlingErrors = append(crawlingErrors, CrawlingError{Url: data.Url, ErrorMessage: data.ErrorLog})
		}
		app.Logger.Summary("Error count: %d", len(errData))
	}

	dataCountInt, _ := strconv.Atoi(app.GetDataCount(config.OriginCollection))
	summary := CrawlingSummary{
		SiteID:         app.Name,
		CollectionName: config.OriginCollection,
		DataCount:      int32(dataCountInt),
		ErrorCount:     int32(len(errData)),
		Errors:         crawlingErrors,
	}
	payloadBytes, err := json.Marshal(summary)
	if err != nil {
		app.Logger.Error("Failed to marshal summary report: %v", err)
	}

	payload := string(payloadBytes)
	if err := app.AddCrawSummary(payload); err != nil {
		app.Logger.Error("Failed to send crawling summary to Crawl Manager API: %v", err)
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
func (app *Crawler) syncRequestMetrics() {
	atomic.AddInt32(&app.requestMetrics.MinuteReqCount, 1) // Increment minute requests
	atomic.AddInt32(&app.requestMetrics.HourlyReqCount, 1) // Increment hourly requests
	atomic.AddInt32(&app.requestMetrics.DayReqCount, 1)    // Increment daily requests
	atomic.AddInt32(&app.requestMetrics.ReqCount, 1)       // Increment total requests
}
func (app *Crawler) startPerformanceTracking() {
	go func() {
		minuteTicker := time.NewTicker(1 * time.Minute)
		hourTicker := time.NewTicker(1 * time.Hour)
		dayTicker := time.NewTicker(24 * time.Hour)
		defer minuteTicker.Stop()
		defer hourTicker.Stop()
		defer dayTicker.Stop()

		for {
			select {
			case <-minuteTicker.C:
				app.requestMetrics.ReportMutex.Lock()
				minuteRate := atomic.LoadInt32(&app.requestMetrics.MinuteReqCount)
				app.Logger.Info("Requests per minute: %d", minuteRate)
				atomic.StoreInt32(&app.requestMetrics.MinuteReqCount, 0) // Reset minute counter
				app.requestMetrics.ReportMutex.Unlock()

			case <-hourTicker.C:
				app.requestMetrics.ReportMutex.Lock()
				hourRate := atomic.LoadInt32(&app.requestMetrics.HourlyReqCount)
				app.Logger.Info("Requests per hour: %d", hourRate)
				atomic.StoreInt32(&app.requestMetrics.HourlyReqCount, 0) // Reset hourly counter
				app.requestMetrics.ReportMutex.Unlock()

			case <-dayTicker.C:
				app.requestMetrics.ReportMutex.Lock()
				dayRate := atomic.LoadInt32(&app.requestMetrics.DayReqCount)
				app.Logger.Info("Requests per day: %d", dayRate)
				atomic.StoreInt32(&app.requestMetrics.DayReqCount, 0) // Reset daily counter
				app.requestMetrics.ReportMutex.Unlock()
			}
		}
	}()
}
func (app *Crawler) startMonitoringReport() {
	go func() {
		ticker := time.NewTicker(60 * time.Minute)
		defer ticker.Stop()

		for {
			<-ticker.C
			app.requestMetrics.ReportMutex.Lock()

			// Calculate metrics
			totalRequests := atomic.LoadInt32(&app.requestMetrics.ReqCount)
			failedRequests := atomic.LoadInt32(&app.requestMetrics.FailedCount)
			successRate := 0.0
			if totalRequests > 0 {
				successRate = float64(totalRequests-failedRequests) / float64(totalRequests) * 100
			}
			elapsed := time.Since(app.StartTime)

			minuteRate := atomic.LoadInt32(&app.requestMetrics.MinuteReqCount)
			hourRate := atomic.LoadInt32(&app.requestMetrics.HourlyReqCount)
			dayRate := atomic.LoadInt32(&app.requestMetrics.DayReqCount)
			// Calculate monthly forecast (30 days)
			monthlyForecast := int32(hourRate) * 24 * 30
			type CrawlingPerformance struct {
				SiteID            string `json:"site_id" bson:"site_id"`
				TotalRequests     int32  `json:"total_requests" bson:"total_requests"`
				FailedRequests    int32  `json:"failed_requests" bson:"failed_requests"`
				SuccessRate       int32  `json:"success_rate" bson:"success_rate"`
				ElapsedTime       int32  `json:"elapsed_time" bson:"elapsed_time"`
				RequestsPerMinute int32  `json:"requests_per_minute" bson:"requests_per_minute"`
				RequestsPerHour   int32  `json:"requests_per_hour" bson:"requests_per_hour"`
				RequestsPerDay    int32  `json:"requests_per_day" bson:"requests_per_day"`
				MonthlyForecast   int32  `json:"monthly_forecast" bson:"monthly_forecast"`
			}
			var report CrawlingPerformance
			report.SiteID = app.Name
			report.TotalRequests = totalRequests
			report.FailedRequests = failedRequests
			report.SuccessRate = int32(successRate)
			report.ElapsedTime = int32(elapsed.Seconds())
			report.RequestsPerMinute = minuteRate
			report.RequestsPerHour = hourRate
			report.RequestsPerDay = dayRate
			report.MonthlyForecast = monthlyForecast

			payloadBytes, err := json.Marshal(report)
			if err != nil {
				app.Logger.Error("Failed to marshal monitoring report: %v", err)
				app.requestMetrics.ReportMutex.Unlock()
				continue
			}

			payload := string(payloadBytes)

			// Log locally and send to API

			// Build a structured report
			log := fmt.Sprintf(`
                ========== Monitoring Report [Hourly] ===========
                | Metric                | Value                  |
                |-----------------------|------------------------|
                | Total Requests        | %d                     |
                | Failed Requests       | %d                     |
                | Success Rate          | %.2f                 |
                | Elapsed Time          | %v                     |
                | Requests Per Minute   | %d                     |
                | Requests Per Hour     | %d                     |
                | Requests Per Day      | %d                     |
                | MonthlyForecast       | %d                     |
                ===================================================
            `, totalRequests, failedRequests, successRate, elapsed, minuteRate, hourRate, dayRate, monthlyForecast)

			// Log the consolidated report
			app.Logger.monitoring(log)
			if err = app.AddCrawlingLog(payload); err != nil {
				app.Logger.Error("Failed to send monitoring report to Crawl Manager API: %v", err)
			}

			// Reset counters if required
			app.requestMetrics.ReportMutex.Unlock()
		}
	}()
}
