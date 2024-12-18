package ninjacrawler

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/playwright-community/playwright-go"
)

// GetPlaywright initializes and runs the Playwright framework.
// It returns a Playwright instance if successful, otherwise returns an error.
func (app *Crawler) GetPlaywright() (*playwright.Playwright, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}
	return pw, nil
}

// GetBrowserPage launches a browser instance and creates a new page using the Playwright framework.
// It supports Chromium, Firefox, and WebKit browsers, and can configure Proxy settings if provided.
// It returns the browser and page instances, or an error if the operation fails.
func (app *Crawler) GetBrowserPage(pw *playwright.Playwright, browserType string, proxy Proxy) (playwright.Browser, playwright.Page, error) {
	var browser playwright.Browser
	var err error

	var browserTypeLaunchOptions playwright.BrowserTypeLaunchOptions
	browserTypeLaunchOptions.Headless = playwright.Bool(!app.isLocalEnv)
	browserTypeLaunchOptions.Devtools = playwright.Bool(app.isLocalEnv)
	// Set additional launch arguments
	if len(app.engine.Args) > 0 {
		browserTypeLaunchOptions.Args = app.engine.Args
	}
	if len(app.engine.ProxyServers) > 0 && proxy.Server != "" {
		// Set Proxy options
		browserTypeLaunchOptions.Proxy = &playwright.Proxy{
			Server:   proxy.Server,
			Username: playwright.String(proxy.Username),
			Password: playwright.String(proxy.Password),
		}
	}
	switch browserType {
	case "chromium":
		browser, err = pw.Chromium.Launch(browserTypeLaunchOptions)
	case "firefox":
		browser, err = pw.Firefox.Launch(browserTypeLaunchOptions)
	case "webkit":
		browser, err = pw.WebKit.Launch(browserTypeLaunchOptions)
	default:
		return nil, nil, fmt.Errorf("unsupported browser type: %s", browserType)
	}

	if err != nil {
		return nil, nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	page, err := browser.NewPage(playwright.BrowserNewPageOptions{
		UserAgent:         playwright.String(app.userAgent),
		JavaScriptEnabled: playwright.Bool(app.engine.JavaScriptEnabled),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create page: %w", err)
	}

	// Conditionally intercept and block resources based on configuration
	if app.engine.BlockResources {
		err := page.Route("**/*", func(route playwright.Route) {
			req := route.Request()
			resourceType := req.ResourceType()
			url := req.URL()

			// Check if the resource should be blocked based on resource type or URL
			if app.shouldBlockResource(resourceType, url) {
				route.Abort()
			} else {
				route.Continue()
			}
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to set up request interception: %w", err)
		}
	}

	return browser, page, nil
}
func (app *Crawler) GetBrowser(pw *playwright.Playwright, browserType string, proxy Proxy) (playwright.BrowserContext, error) {
	var (
		browser               playwright.Browser
		err                   error
		browserTypeLaunchOpts = playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(!app.isLocalEnv),
			Devtools: playwright.Bool(app.isLocalEnv),
			Args:     app.engine.Args,
		}
		contextOpts = playwright.BrowserNewContextOptions{
			ExtraHttpHeaders: map[string]string{
				"Accept-Language": "ja-JP,ja;q=0.9",
			},
		}
	)

	// Set proxy options if available
	if len(app.engine.ProxyServers) > 0 && proxy.Server != "" {
		browserTypeLaunchOpts.Proxy = &playwright.Proxy{
			Server:   proxy.Server,
			Username: playwright.String(proxy.Username),
			Password: playwright.String(proxy.Password),
		}
	}
	// Launch the appropriate browser and configure user-agent headers
	switch browserType {
	case "chromium":
		browser, err = pw.Chromium.Launch(browserTypeLaunchOpts)
		setChromiumHeaders(&contextOpts, browser)
	case "firefox":
		browser, err = pw.Firefox.Launch(browserTypeLaunchOpts)
		setFirefoxHeaders(&contextOpts, browser)
	case "webkit":
		browser, err = pw.WebKit.Launch(browserTypeLaunchOpts)
		setWebKitHeaders(&contextOpts, browser)
	default:
		return nil, fmt.Errorf("unsupported browser type: %s", browserType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	// Overwrite the default User-Agent header
	if app.userAgent != "" {
		contextOpts.ExtraHttpHeaders["User-Agent"] = app.userAgent
	}
	context, err := browser.NewContext(contextOpts)
	if err != nil {
		return nil, fmt.Errorf("could not create new browser context: %w", err)
	}

	if len(app.engine.Cookies) > 0 {
		err = context.AddCookies(app.engine.Cookies)
		if err != nil {
			return nil, fmt.Errorf("failed to add cookies: %w", err)
		}
	}

	return context, nil
}

func (app *Crawler) GetPage(context playwright.BrowserContext) (playwright.Page, error) {

	page, err := context.NewPage()
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}

	// Conditionally intercept and block resources based on configuration
	if app.engine.BlockResources {
		err := page.Route("**/*", func(route playwright.Route) {
			req := route.Request()
			resourceType := req.ResourceType()
			url := req.URL()

			// Check if the resource should be blocked based on resource type or URL
			if app.shouldBlockResource(resourceType, url) {
				route.Abort()
			} else {
				route.Continue()
			}
		})
		if err != nil {
			return nil, fmt.Errorf("failed to set up request interception: %w", err)
		}
	}
	return page, nil
}

// NavigateToURL navigates to a specified URL using the given Playwright page.
// It waits until the page is fully loaded, handles cookie consent, and returns a goquery document representing the DOM.
// If navigation or handling consent fails, it logs the page content to a file and returns an error.
func (app *Crawler) NavigateToURL(pageInterFace interface{}, url string, proxy Proxy) (playwright.Page, *goquery.Document, error) {
	var page playwright.Page
	page = pageInterFace.(playwright.Page)
	originalURL := url // Store the original URL for comparison
	pageGotoOptions := playwright.PageGotoOptions{
		Timeout: playwright.Float(float64(app.engine.Timeout.Milliseconds())),
	}
	if app.engine.WaitForDynamicRendering && app.engine.WaitForSelector == nil {
		pageGotoOptions.WaitUntil = playwright.WaitUntilStateNetworkidle
	}

	// Navigate to the URL
	res, err := page.Goto(url, pageGotoOptions)
	if err != nil {
		d, e := app.handleProxyError(proxy, err)
		return nil, d, e
	}
	if !res.Ok() {
		return nil, nil, app.handleHttpError(res.Status(), res.StatusText(), url, page)
	}

	// Check for redirection
	finalURL := page.URL()
	if originalURL != finalURL && *app.engine.TrackRedirection {
		app.CurrentUrl = finalURL
		_ = app.updateRedirection(originalURL, finalURL)
		app.Logger.Warn(fmt.Sprintf("Redirection detected: %s -> %s", originalURL, finalURL))
	}

	// Handle cookie consent
	if err = handleCookieConsent(page, app.engine.CookieConsent); err != nil {
		if app.engine.CookieConsent.IsOptional {
			app.Logger.Warn("cookie consent not found: %s", err.Error())
		} else {
			app.Logger.Html(app.getHtmlFromPage(page), url, err.Error())
			return nil, nil, fmt.Errorf("failed to handle cookie consent: %w", err)
		}
	}
	// Handle mouse simulation, if applicable
	if app.engine.SimulateMouse != nil && *app.engine.SimulateMouse {
		if mError := autoMoveMouse(page); mError != nil {
			app.Logger.Error("Mouse Simulate Error: %s", mError.Error())
		}
	}

	// Wait for selector if applicable
	if app.engine.WaitForSelector != nil || app.engine.WaitForSelectorVisible != nil {
		selector := ""
		if app.engine.WaitForSelector != nil {
			selector = *app.engine.WaitForSelector
		}
		pageWaitForSelectorOptions := playwright.PageWaitForSelectorOptions{
			Timeout: playwright.Float(float64(app.engine.Timeout.Milliseconds())),
			State:   playwright.WaitForSelectorStateAttached,
		}
		if app.engine.WaitForSelectorVisible != nil {
			pageWaitForSelectorOptions.State = playwright.WaitForSelectorStateVisible
			pageWaitForSelectorOptions.Timeout = playwright.Float(float64(app.engine.Timeout.Milliseconds()))
			selector = *app.engine.WaitForSelectorVisible
		}
		_, err = page.WaitForSelector(selector, pageWaitForSelectorOptions)
		if err != nil {
			app.Logger.Html(app.getHtmlFromPage(page), url, fmt.Sprintf("Failed to find %s: %s", selector, err.Error()))
			if *app.engine.IsWaitForSelectorOptional {
				app.Logger.Warn("%s Not found found in DOM: %s", selector, err.Error())
			} else {
				return nil, nil, fmt.Errorf("failed to find %s: %w", selector, err)
			}
		}
	}

	document, err := app.GetDocument(page)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get page DOM: %w", err)
	}

	if app.engine.SendHtmlToBigquery != nil && *app.engine.SendHtmlToBigquery {
		sendErr := app.SendHtmlToBigquery(document, url)
		if sendErr != nil {
			app.Logger.Error("SendHtmlToBigquery Error: %s", sendErr.Error())
		}
	}
	if *app.engine.StoreHtml {
		if StoreHtmlErr := app.SaveHtml(document, url); StoreHtmlErr != nil {
			app.Logger.Error(StoreHtmlErr.Error())
		}
	}
	return page, document, err
}
