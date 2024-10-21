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
	//fmt.Println(l.FormatArgs())
	url, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("error launching browser: %s", err.Error())
	}
	browser := rod.New().ControlURL(url).MustConnect()
	// Optionally handle proxy authentication
	if proxy.Username != "" && proxy.Password != "" {
		go func() {
			errAuth := browser.HandleAuth(proxy.Username, proxy.Password)()
			if errAuth != nil {
				app.Logger.Error("Error handling proxy authentication: %s", errAuth.Error())
			}
		}()
	}

	return browser, nil
}

// GetRodPage creates a new page using the Rod framework.
// It returns the page instance, or an error if the operation fails.
func (app *Crawler) GetRodPage(browser *rod.Browser) (*rod.Page, error) {
	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, fmt.Errorf("error creating page: %s", err.Error())
	}

	err = page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent: app.userAgent,
	})
	if err != nil {
		return nil, fmt.Errorf("error setting user agent: %s", err.Error())
	}

	return page, nil
}

// NavigateRodURL navigates to a specified URL using the Rod page.
// It waits until the page is fully loaded, handles cookie consent, and returns the page DOM.
func (app *Crawler) NavigateRodURL(page *rod.Page, url string) (*rod.Page, *goquery.Document, error) {
	e := proto.NetworkResponseReceived{}
	wait := page.WaitEvent(&e)
	// Go to the URL with a timeout
	pageWithTimeout := page.Timeout(app.engine.Timeout)
	err := pageWithTimeout.Navigate(url)
	if err != nil {
		d, e := app.handleProxyError(err)
		return nil, d, e
	}
	wait()
	if e.Response == nil {
		return nil, nil, fmt.Errorf("no response received: %+v", e)
	}
	if !Ok(e.Response.Status) {
		return nil, nil, app.handleHttpError(e.Response.Status, e.Response.StatusText, url, page)
	}

	// Wait for selector if applicable
	if app.engine.WaitForSelector != nil {
		elm, navErr := page.Timeout(app.engine.Timeout).Element(*app.engine.WaitForSelector)
		if navErr != nil {
			msg := fmt.Sprintf("element not found: %s", navErr.Error())
			html, htmlErr := page.HTML()
			if htmlErr != nil {
				html = "could not get html"
				msg = fmt.Sprintf("Html Parse Failed: %s", htmlErr.Error())
			}
			app.Logger.Html(html, url, msg)
			return nil, nil, fmt.Errorf("element not found: %s", navErr.Error())
		}
		if elm == nil {
			err = page.WaitStable(app.engine.Timeout)
			if err != nil {
				return nil, nil, fmt.Errorf("page did not stabilize: %w", err)
			}
			return nil, nil, fmt.Errorf("element not found: %s", url)
		}
	} else {
		if errLoad := page.WaitLoad(); errLoad != nil {
			return nil, nil, errLoad
		}
	}

	// Handle cookie consent
	if err = handleCookieConsent(page, app.engine.CookieConsent); err != nil {
		html, _ := app.GetHtml(page)
		app.Logger.Html(html, url, err.Error())
		return nil, nil, err
	}

	// Get the page DOM
	document, domErr := app.GetDocument(page)
	if domErr != nil {
		return nil, nil, domErr
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
	return page, document, nil
}
func Ok(Status int) bool {
	return Status == 0 || (Status >= 200 && Status <= 299)
}
