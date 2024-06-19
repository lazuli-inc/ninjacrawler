package ninjacrawler

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/playwright-community/playwright-go"
)

// GetPlaywright initializes and runs the Playwright framework.
// It returns a Playwright instance if successful, otherwise returns an error.
func GetPlaywright() (*playwright.Playwright, error) {
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
	browserTypeLaunchOptions.Headless = playwright.Bool(!isLocalEnv(app.Config.GetString("APP_ENV")))
	browserTypeLaunchOptions.Devtools = playwright.Bool(isLocalEnv(app.Config.GetString("APP_ENV")))

	if len(app.engine.ProxyServers) > 0 {
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
		UserAgent: playwright.String(app.Config.GetString("USER_AGENT")),
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

// HandleCookieConsent attempts to fill form fields and click the specified cookie consent button on a cookie consent dialog.
// It logs an error if the button cannot be found or clicked.
func (app *Crawler) HandleCookieConsent(page playwright.Page) error {
	action := app.engine.CookieConsent
	if action == nil {
		return nil
	}
	// Check and fill form fields if any
	if len(action.Fields) > 0 {
		for _, field := range action.Fields {
			inputSelector := fmt.Sprintf("input[name='%s']", field.Key)
			input, err := page.QuerySelector(inputSelector)
			if err != nil {
				return fmt.Errorf("failed to find input field with name '%s': %w", field.Key, err)
			}
			if input != nil {
				err = input.Fill(field.Val)
				if err != nil {
					return fmt.Errorf("failed to fill input field with name '%s': %w", field.Key, err)
				}
			}
		}

	}
	if action.ButtonText != "" {
		// Construct the selector for the button
		buttonSelector := fmt.Sprintf("button:has-text('%s')", action.ButtonText)
		page.WaitForSelector(buttonSelector)
		button, err := page.QuerySelector(buttonSelector)
		if err == nil && button != nil {
			err = button.Click()
			if err != nil {
				return fmt.Errorf("failed to click cookie consent button: %w", err)
			}

			page.WaitForSelector(action.MustHaveSelectorAfterAction)

		}
	}

	return nil
}

// NavigateToURL navigates to a specified URL using the given Playwright page.
// It waits until the page is fully loaded, handles cookie consent, and returns a goquery document representing the DOM.
// If navigation or handling consent fails, it logs the page content to a file and returns an error.
func (app *Crawler) NavigateToURL(page playwright.Page, url string) (*goquery.Document, error) {
	waitUntil := playwright.WaitUntilStateDomcontentloaded
	if app.engine.IsDynamic {
		waitUntil = playwright.WaitUntilStateNetworkidle
	}

	_, err := page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: waitUntil,
		Timeout:   playwright.Float(float64(app.engine.Timeout)),
	})
	if err != nil {
		app.Logger.Html(page, err.Error())
		return nil, err
	}

	// Handle cookie consent
	err = app.HandleCookieConsent(page)
	if err != nil {
		app.Logger.Html(page, err.Error())
		return nil, err
	}
	return app.GetPageDom(page)
}
