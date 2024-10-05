package ninjacrawler

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/go-rod/rod/lib/proto"
)

// GetRodBrowser initializes and runs Rod browser.
// It returns a Rod browser instance if successful, otherwise returns an error.
func (app *Crawler) GetRodBrowser(proxy Proxy) (*rod.Browser, error) {
	// Setup the browser launcher with proxy if provided
	l := launcher.New().Headless(!app.isLocalEnv).Devtools(app.isLocalEnv).NoSandbox(!app.isLocalEnv)

	if len(app.engine.ProxyServers) > 0 && proxy.Server != "" {
		l = l.Set(flags.ProxyServer, proxy.Server)
	}

	url := l.MustLaunch()
	browser := rod.New().ControlURL(url).MustConnect()
	// Optionally handle proxy authentication
	if proxy.Username != "" && proxy.Password != "" {
		go browser.MustHandleAuth(proxy.Username, proxy.Password)()
	}

	return browser, nil
}

// GetRodPage creates a new page using the Rod framework.
// It returns the page instance, or an error if the operation fails.
func (app *Crawler) GetRodPage(browser *rod.Browser) (*rod.Page, error) {
	page := browser.MustPage()

	err := page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent: app.userAgent,
	})
	if err != nil {
		return nil, fmt.Errorf("error setting user agent: %s", err.Error())
	}

	return page, nil
}

// NavigateRodURL navigates to a specified URL using the Rod page.
// It waits until the page is fully loaded, handles cookie consent, and returns the page DOM.
func (app *Crawler) NavigateRodURL(page *rod.Page, url string) (*goquery.Document, error) {
	e := proto.NetworkResponseReceived{}
	wait := page.WaitEvent(&e)
	// Go to the URL with a timeout
	err := page.Timeout(app.engine.Timeout).Navigate(url)
	if err != nil {
		return nil, err
	}
	wait()
	if e.Response == nil {
		return nil, fmt.Errorf("no response received: %+v", e)
	}
	if !Ok(e.Response.Status) {
		return nil, app.handleHttpError(e.Response.Status, e.Response.StatusText, url, page)
	}

	// Optionally wait for a specific selector
	if app.engine.WaitForSelector != nil {
		elm, navErr := page.Timeout(app.engine.Timeout).Element(*app.engine.WaitForSelector)
		if navErr != nil {
			return nil, fmt.Errorf("element not found: %s", navErr.Error())
		}
		if elm == nil {
			err = page.WaitStable(app.engine.Timeout)
			if err != nil {
				return nil, fmt.Errorf("page did not stabilize: %w", err)
			}
			return nil, fmt.Errorf("element not found: %s", url)
		}
	} else {
		page.MustWaitLoad()
	}

	// Handle cookie consent
	err = app.HandleRodCookieConsent(page)
	if err != nil {
		app.Logger.Html(page.MustHTML(), url, err.Error())
		return nil, err
	}

	// Get the page DOM
	document, domErr := app.GetRodPageDom(page)
	if domErr != nil {
		return nil, domErr
	}

	// Optionally send HTML to BigQuery or store it
	if app.engine.SendHtmlToBigquery != nil && *app.engine.SendHtmlToBigquery {
		sendErr := app.SendHtmlToBigquery(document, url)
		if sendErr != nil {
			app.Logger.Fatal("SendHtmlToBigquery Error: %s", sendErr.Error())
		}
	}
	if *app.engine.StoreHtml {
		if err := app.SaveHtml(document, url); err != nil {
			app.Logger.Error(err.Error())
		}
	}
	return document, nil
}

// HandleRodCookieConsent handles cookie consent dialogs on the page.
func (app *Crawler) HandleRodCookieConsent(page *rod.Page) error {
	action := app.engine.CookieConsent
	if action == nil {
		return nil
	}

	// Fill input fields if specified
	for _, field := range action.Fields {
		el := page.MustElement(fmt.Sprintf("input[name='%s']", field.Key))
		el.MustInput(field.Val)
	}

	// Click the consent button
	if action.ButtonText != "" {
		button := page.MustElementR("button", action.ButtonText)
		button.MustClick()

		page.MustWaitLoad()
	}

	return nil
}
func Ok(Status int) bool {
	return Status == 0 || (Status >= 200 && Status <= 299)
}
