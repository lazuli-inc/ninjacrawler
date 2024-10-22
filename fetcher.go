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

	crawlerCtx := app.getCrawlerCtx(navigationContext)
	crawlerCtx.UrlCollection = urlCollection
	if navigateToApi {
		crawlerCtx.ApiResponse = navigationContext.Response.(Map)
	}
	return crawlerCtx, nil
}

func (app *Crawler) getCrawlerCtx(navigationContext *NavigationContext) *CrawlerContext {
	crawlerCtx := &CrawlerContext{
		App:      app,
		Document: navigationContext.Document,
	}

	if *app.engine.IsDynamic {
		if *app.engine.Adapter == PlayWrightEngine {
			crawlerCtx.Page = navigationContext.Response.(playwright.Page)
		} else {
			crawlerCtx.RodPage = navigationContext.Response.(*rod.Page)
		}
	}
	return crawlerCtx
}
func (app *Crawler) navigateTo(ctx context.Context, crawlableUrl string, origin string, navigateToApi bool, currentProxy Proxy) (*NavigationContext, error) {
	var (
		err      error
		doc      *goquery.Document
		pwPage   playwright.Page
		rdPage   *rod.Page
		response interface{}
	)

	// Simplified logging for proxy usage
	if currentProxy.Server != "" {
		app.Logger.Info("Crawling %s: %s using Proxy %s", origin, crawlableUrl, currentProxy.Server)
	} else {
		app.Logger.Info("Crawling %s: %s", origin, crawlableUrl)
	}

	// Check if the context is already done (canceled or timed out)
	select {
	case <-ctx.Done():
		app.Logger.Warn("Navigation canceled or timed out for %s", crawlableUrl)
		return nil, fmt.Errorf("navigation timeout: %s", crawlableUrl)
	default:
		// Proceed with navigation
	}

	// Handle dynamic or static crawling
	if *app.engine.IsDynamic {
		switch *app.engine.Adapter {
		case PlayWrightEngine:
			pwPage, doc, err = app.NavigateToURL(app.pwPage, crawlableUrl)
			response = pwPage
		case RodEngine:
			rdPage, doc, err = app.NavigateRodURL(app.rdPage, crawlableUrl)
			response = rdPage
		default:
			return nil, fmt.Errorf("unsupported engine adapter: %v", *app.engine.Adapter)
		}
	} else if navigateToApi {
		// Navigate using API
		response, err = app.NavigateToApiURL(app.httpClient, crawlableUrl, currentProxy)
	} else {
		// Static navigation
		doc, err = app.NavigateToStaticURL(app.httpClient, crawlableUrl, currentProxy)
		response = app.httpClient
	}

	// Check for errors after navigation
	if err != nil {
		app.Logger.Error("Error during navigation to %s: %v", crawlableUrl, err)
		return nil, err
	}

	// Return the navigation context
	return &NavigationContext{
		Document: doc,
		Response: response,
	}, nil
}
