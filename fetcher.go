package ninjacrawler

import (
	"context"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
	"github.com/playwright-community/playwright-go"
)

func (app *Crawler) handleCrawlWorker(processorConfig ProcessorConfig, urlCollection UrlCollection, proxy Proxy) (*CrawlerContext, error) {
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

	// Add a timeout for the navigation process
	ctx, cancel := context.WithTimeout(context.Background(), app.engine.Timeout*2)
	defer cancel()

	navigationContext, err := app.navigateTo(ctx, crawlableUrl, processorConfig.OriginCollection, navigateToApi, proxy)
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

func (app *Crawler) navigateTo(ctx context.Context, crawlableUrl string, origin string, navigateToApi bool, currentProxy Proxy) (*NavigationContext, error) {
	var err error
	var doc *goquery.Document
	var response interface{}
	if currentProxy.Server != "" {
		app.Logger.Info("Crawling :%s: %s using Proxy %s", origin, crawlableUrl, currentProxy.Server)
	} else {
		app.Logger.Info("Crawling :%s: %s", origin, crawlableUrl)
	}
	done := make(chan struct{})
	go func() {
		if *app.engine.IsDynamic {
			if *app.engine.Adapter == PlayWrightEngine {
				doc, err = app.NavigateToURL(app.pwPage, crawlableUrl)
				response = app.pwPage
			} else {
				doc, err = app.NavigateRodURL(app.rdPage, crawlableUrl)
				response = app.rdPage
			}
		} else if navigateToApi {
			response, err = app.NavigateToApiURL(app.httpClient, crawlableUrl, currentProxy)
		} else {
			doc, err = app.NavigateToStaticURL(app.httpClient, crawlableUrl, currentProxy)
			response = app.httpClient
		}
		close(done)
	}()

	select {
	case <-done:
		if err != nil {
			return nil, err
		}
		return &NavigationContext{
			Document: doc,
			Response: response,
		}, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("navigation timeout: %s", crawlableUrl)
	}
}
