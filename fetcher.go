package ninjacrawler

import (
	"context"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
	"github.com/playwright-community/playwright-go"
)

func (app *Crawler) handleCrawlWorker(page interface{}, processorConfig ProcessorConfig, urlCollection UrlCollection, proxy Proxy) (*CrawlerContext, error) {

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

	navigationContext, navErr := app.navigateTo(ctx, page, crawlableUrl, processorConfig.OriginCollection, navigateToApi, proxy)
	if navErr != nil {
		return nil, navErr
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
func (app *Crawler) navigateTo(ctx context.Context, page interface{}, crawlableUrl string, origin string, navigateToApi bool, currentProxy Proxy) (*NavigationContext, error) {
	// Create a channel to capture navigation result
	resultChan := make(chan navigationResult, 1)

	go func() {
		var (
			err      error
			doc      *goquery.Document
			pwPage   playwright.Page
			rdPage   *rod.Page
			response interface{}
		)
		if currentProxy.Server != "" {
			app.Logger.Info("Crawling %s: %s using Proxy %s", origin, crawlableUrl, currentProxy.Server)
		} else {
			app.Logger.Info("Crawling %s: %s", origin, crawlableUrl)
		}
		// Actual navigation logic
		if *app.engine.IsDynamic && page != nil {
			if *app.engine.Adapter == PlayWrightEngine {
				pwPage, doc, err = app.NavigateToURL(page, crawlableUrl, currentProxy)
				response = pwPage
			} else if *app.engine.Adapter == RodEngine {
				rdPage, doc, err = app.NavigateRodURL(page, crawlableUrl, currentProxy)
				response = rdPage
			}
		} else if navigateToApi {
			response, err = app.NavigateToApiURL(app.httpClient, crawlableUrl, currentProxy)
		} else {
			doc, err = app.NavigateToStaticURL(app.httpClient, crawlableUrl, currentProxy)
			response = app.httpClient
		}

		resultChan <- navigationResult{
			NavigationContext: &NavigationContext{
				Document: doc,
				Response: response,
			},
			Err: err,
		}
	}()

	// Wait for either navigation completion or context timeout
	select {
	case result := <-resultChan:
		if result.Err != nil {
			app.Logger.Error("Error during navigation to %s: %v", crawlableUrl, result.Err)
			return nil, result.Err
		}
		return result.NavigationContext, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("navigation timeout: %s", crawlableUrl)
	}
}

// Helper struct to capture navigation result
type navigationResult struct {
	NavigationContext *NavigationContext
	Err               error
}
