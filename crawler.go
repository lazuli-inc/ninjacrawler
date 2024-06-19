package ninjacrawler

import (
	"context"
	"sync"
	"sync/atomic"
)

// Struct to hold both results and the UrlCollection
type CrawlResult struct {
	Results       interface{}
	UrlCollection UrlCollection
}

func (app *Crawler) crawlWorker(ctx context.Context, dbCollection string, urlChan <-chan UrlCollection, resultChan chan<- interface{}, proxy Proxy, processor interface{}, isLocalEnv bool, counter *int32) {
	browser, page, err := app.GetBrowserPage(app.pw, app.engine.BrowserType, proxy)
	if err != nil {
		app.Logger.Fatal("failed to initialize browser with Proxy: %v\n", err)
	}
	defer browser.Close()
	defer page.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case urlCollection, more := <-urlChan:
			if !more {
				return
			}

			if isLocalEnv && atomic.LoadInt32(counter) >= int32(app.engine.DevCrawlLimit) {
				//app.Logger.Info("Dev Crawl limit reached!")
				return
			}

			if proxy.Server != "" {
				app.Logger.Info("Crawling %s using Proxy %s", urlCollection.Url, proxy.Server)
			} else {
				app.Logger.Info("Crawling %s", urlCollection.Url)
			}

			doc, err := app.NavigateToURL(page, urlCollection.Url)
			if err != nil {
				err := app.markAsError(urlCollection.Url, dbCollection)
				if err != nil {
					app.Logger.Info(err.Error())
					return
				}
				continue
			}

			crawlerCtx := CrawlerContext{
				App:           app,
				Document:      doc,
				UrlCollection: urlCollection,
				Page:          page,
			}

			var results interface{}
			switch v := processor.(type) {
			case func(CrawlerContext) []UrlCollection:
				results = v(crawlerCtx)

			case UrlSelector:
				results = app.processDocument(doc, v, urlCollection)

			case ProductDetailSelector:
				results = crawlerCtx.handleProductDetail()

			default:
				app.Logger.Fatal("Unsupported processor type: %T", processor)
			}

			crawlResult := CrawlResult{
				Results:       results,
				UrlCollection: urlCollection,
			}

			select {
			case resultChan <- crawlResult:
				atomic.AddInt32(counter, 1)
			default:
				app.Logger.Info("Channel is full, dropping Item")
			}
		}
	}
}

func (app *Crawler) CrawlUrls(collection string, processor interface{}) {
	var items []UrlCollection
	for {

		urlCollections := app.getUrlCollections(collection)
		if len(urlCollections) == 0 {
			break
		}

		var wg sync.WaitGroup
		urlChan := make(chan UrlCollection, len(urlCollections))
		resultChan := make(chan interface{}, len(urlCollections))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		counter := int32(0)

		for _, urlCollection := range urlCollections {
			urlChan <- urlCollection
		}
		close(urlChan)

		proxyCount := len(app.engine.ProxyServers)
		batchSize := app.engine.ConcurrentLimit
		totalUrls := len(urlCollections)
		goroutineCount := min(max(proxyCount, 1)*batchSize, totalUrls)

		for i := 0; i < goroutineCount; i++ {
			proxy := Proxy{}
			if proxyCount > 0 {
				proxy = app.engine.ProxyServers[i%proxyCount]
			}
			wg.Add(1)
			go func(proxy Proxy) {
				defer wg.Done()
				app.crawlWorker(ctx, collection, urlChan, resultChan, proxy, processor, isLocalEnv(app.Config.GetString("APP_ENV")), &counter)
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
					items = append(items, res...)
					for _, item := range res {
						if item.Parent == "" && collection != baseCollection {
							app.Logger.Fatal("Missing Parent Url")
							continue
						}
					}
					app.insert(res, v.UrlCollection.Url)
					err := app.markAsComplete(v.UrlCollection.Url, collection)
					if err != nil {
						app.Logger.Error(err.Error())
						continue
					}
					app.Logger.Info("(%d) :%s: Found From [%s => %s]", len(res), app.collection, collection, v.UrlCollection.Url)
				}
			}

			if isLocalEnv(app.Config.GetString("APP_ENV")) && atomic.LoadInt32(&counter) >= int32(app.engine.DevCrawlLimit) {
				cancel()
				break
			}
		}

	}

	app.Logger.Info("Total :%s: = (%d)", app.collection, len(items))

}

// CrawlPageDetail initiates the crawling process for detailed page information from the specified collection.
// It distributes the work among multiple goroutines and uses proxies if available.
func (app *Crawler) CrawlPageDetail(collection string) {
	urlCollections := app.getUrlCollections(collection)

	var wg sync.WaitGroup
	urlChan := make(chan UrlCollection, len(urlCollections))
	resultChan := make(chan interface{}, len(urlCollections))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	counter := int32(0)
	for _, urlCollection := range urlCollections {
		urlChan <- urlCollection
	}
	close(urlChan)

	proxyCount := len(app.engine.ProxyServers)
	batchSize := app.engine.ConcurrentLimit
	totalUrls := len(urlCollections)
	goroutineCount := min(max(proxyCount, 1)*batchSize, totalUrls) // Determine the required number of goroutines

	for i := 0; i < goroutineCount; i++ {
		proxy := Proxy{}
		if proxyCount > 0 {
			proxy = app.engine.ProxyServers[i%proxyCount]
		}
		wg.Add(1)
		go func(proxy Proxy) {
			defer wg.Done()
			app.crawlWorker(ctx, collection, urlChan, resultChan, proxy, app.ProductDetailSelector, isLocalEnv(app.Config.GetString("APP_ENV")), &counter)
		}(proxy)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	total := 0
	for results := range resultChan {
		switch v := results.(type) {
		case CrawlResult:
			switch res := v.Results.(type) {
			case *ProductDetail:
				app.saveProductDetail(res)
				err := app.submitProductData(res)
				if err != nil {
					app.Logger.Fatal("failed to submit product data to API Server: %v", err)
					err := app.markAsError(v.UrlCollection.Url, collection)
					if err != nil {
						app.Logger.Info(err.Error())
						return
					}
				}

				err = app.markAsComplete(v.UrlCollection.Url, collection)
				if err != nil {
					app.Logger.Error(err.Error())
					continue
				}
				total++
				if isLocalEnv(app.Config.GetString("APP_ENV")) && total >= app.engine.DevCrawlLimit {
					break
				}
			}
		}
	}
	exportProductDetailsToCSV(app, app.collection, 1)
	app.Logger.Info("Total %v %v Inserted ", total, app.collection)
}

// PageSelector adds a new URL selector to the crawler.
func (app *Crawler) PageSelector(selector UrlSelector) *Crawler {
	app.UrlSelectors = append(app.UrlSelectors, selector)
	return app
}

// StartUrlCrawling initiates the URL crawling process for all added selectors.
func (app *Crawler) StartUrlCrawling() *Crawler {
	for _, selector := range app.UrlSelectors {
		app.Collection(selector.ToCollection).
			CrawlUrls(selector.FromCollection, selector)
	}
	return app
}
