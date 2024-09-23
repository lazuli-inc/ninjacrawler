package ninjacrawler

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/playwright-community/playwright-go"
)

func (app *Crawler) handleCrawlWorker(processorConfig ProcessorConfig, proxy Proxy, urlCollection UrlCollection) (*CrawlerContext, error) {
	var page playwright.Page
	var browser playwright.Browser
	var err error
	var doc *goquery.Document
	var apiResponse map[string]interface{}
	if *app.engine.IsDynamic {
		browser, page, err = app.GetBrowserPage(app.pw, app.engine.BrowserType, proxy)
		if err != nil {
			app.Logger.Fatal(err.Error())
		}
		defer browser.Close()
		defer page.Close()
	}

	crawlableUrl := urlCollection.Url
	if urlCollection.ApiUrl != "" {
		crawlableUrl = urlCollection.ApiUrl
	}
	if urlCollection.CurrentPageUrl != "" {
		crawlableUrl = urlCollection.CurrentPageUrl
	}
	if proxy.Server != "" {
		app.Logger.Info("Crawling :%s: %s using Proxy %s", processorConfig.OriginCollection, crawlableUrl, proxy.Server)
	} else {
		app.Logger.Info("Crawling :%s: %s", processorConfig.OriginCollection, crawlableUrl)
	}
	if *app.engine.IsDynamic {
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
		return nil, err
	}
	crawlerCtx := &CrawlerContext{
		App:           app,
		Document:      doc,
		UrlCollection: urlCollection,
		Page:          page,
		ApiResponse:   apiResponse,
	}
	return crawlerCtx, nil
}
