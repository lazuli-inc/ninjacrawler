package ninjacrawler

import (
	"github.com/PuerkitoBio/goquery"
)

func (app *Crawler) handleCrawlWorker(processorConfig ProcessorConfig, proxy Proxy, urlCollection UrlCollection) (*CrawlerContext, error) {
	var err error
	var doc *goquery.Document
	var apiResponse map[string]interface{}

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
		if *app.engine.Adapter == PlayWrightEngine {
			doc, err = app.NavigateToURL(app.pwPage, crawlableUrl)
		} else {
			doc, err = app.NavigateRodURL(app.rdPage, crawlableUrl)
		}
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
		Page:          app.pwPage,
		RodPage:       app.rdPage,
		ApiResponse:   apiResponse,
	}
	return crawlerCtx, nil
}
