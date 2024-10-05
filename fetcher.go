package ninjacrawler

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
	"github.com/playwright-community/playwright-go"
)

func (app *Crawler) handleCrawlWorker(processorConfig ProcessorConfig, urlCollection UrlCollection) (*CrawlerContext, error) {
	var err error

	crawlableUrl := urlCollection.Url
	if urlCollection.ApiUrl != "" {
		crawlableUrl = urlCollection.ApiUrl
	}
	if urlCollection.CurrentPageUrl != "" {
		crawlableUrl = urlCollection.CurrentPageUrl
	}
	navigateToApi := urlCollection.ApiUrl != ""
	switch processorConfig.Processor.(type) {
	case ProductDetailApi:
		navigateToApi = true
	default:
	}
	navigationContext, err := app.navigateTo(crawlableUrl, processorConfig.OriginCollection, navigateToApi)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}
	crawlerCtx := &CrawlerContext{
		App:           app,
		Document:      navigationContext.Document,
		UrlCollection: urlCollection,
	}

	if *app.engine.IsDynamic {
		if *app.engine.Adapter == PlayWrightEngine {
			crawlerCtx.Page = navigationContext.Response.(playwright.Page)
		} else {
			crawlerCtx.RodPage = navigationContext.Response.(*rod.Page)
		}
	} else if navigateToApi {
		crawlerCtx.ApiResponse = navigationContext.Response.(Map)
	}
	return crawlerCtx, nil
}

func (app *Crawler) navigateTo(crawlableUrl string, origin string, navigateToApi bool) (*NavigationContext, error) {
	var err error
	var doc *goquery.Document
	var response interface{}
	if app.CurrentProxy.Server != "" {
		app.Logger.Info("Crawling :%s: %s using Proxy %s", origin, crawlableUrl, app.CurrentProxy.Server)
	} else {
		app.Logger.Info("Crawling :%s: %s", origin, crawlableUrl)
	}
	if *app.engine.IsDynamic {
		if *app.engine.Adapter == PlayWrightEngine {
			doc, err = app.NavigateToURL(app.pwPage, crawlableUrl)
			response = app.pwPage
		} else {
			doc, err = app.NavigateRodURL(app.rdPage, crawlableUrl)
			response = app.rdPage
		}
	} else if navigateToApi {
		response, err = app.NavigateToApiURL(app.httpClient, crawlableUrl, app.CurrentProxy)
	} else {
		doc, err = app.NavigateToStaticURL(app.httpClient, crawlableUrl, app.CurrentProxy)
		response = app.httpClient
	}

	if err != nil {
		return nil, err
	}
	return &NavigationContext{
		Document: doc,
		Response: response,
	}, nil
}
