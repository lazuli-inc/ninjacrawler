package ninjacrawler

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type Engine struct {
	ForceInstallPlaywright  bool
	Provider                string // http,playwright,zenrows
	ProviderOption          ProviderQueryOption
	BrowserType             string
	ConcurrentLimit         int
	IsDynamic               *bool
	DevCrawlLimit           int
	StgCrawlLimit           int
	BlockResources          bool
	JavaScriptEnabled       bool
	BlockedURLs             []string
	BoostCrawling           bool
	ProxyServers            []Proxy
	ProxyStrategy           string
	CookieConsent           *CookieAction
	Timeout                 time.Duration
	WaitForDynamicRendering bool
	SleepAfter              int
	MaxRetryAttempts        int
	IgnoreRetryOnValidation *bool
	Args                    []string
	SleepDuration           int
	RetrySleepDuration      int
	ErrorCodes              []int
	CrawlTimeout            int
	WaitForSelector         *string
	StoreHtml               *bool
}
type ProviderQueryOption struct {
	JsRender             bool
	UsePremiumProxyRetry bool
	CustomHeaders        bool
	PremiumProxy         bool
	ProxyCountry         string
	SessionID            int
	Device               string
	OriginalStatus       bool
	AllowedStatusCodes   string
	WaitFor              string
	Wait                 int
	BlockResources       string
	JSONResponse         bool
	CSSExtractor         string // JSON formatted string
	Autoparse            bool
	MarkdownResponse     bool
	Screenshot           bool
	ScreenshotFullPage   bool
	ScreenshotSelector   string
}

func (app *Crawler) SetBrowserType(browserType string) *Crawler {
	app.engine.BrowserType = browserType
	return app
}

func (app *Crawler) SetConcurrentLimit(concurrentLimit int) *Crawler {
	app.engine.ConcurrentLimit = concurrentLimit
	return app
}

func (app *Crawler) IsDynamicPage(isDynamic bool) *Crawler {
	app.engine.IsDynamic = &isDynamic
	app.toggleClient()
	return app
}

func (app *Crawler) SetCrawlLimit(crawlLimit int) *Crawler {
	app.engine.DevCrawlLimit = crawlLimit
	return app
}
func (app *Crawler) SetBlockResources(block bool) *Crawler {
	app.engine.BlockResources = block
	return app
}

func (app *Crawler) EnableBoostCrawling() *Crawler {
	app.engine.BoostCrawling = true
	app.engine.ProxyServers = app.getProxyList()
	return app
}
func (app *Crawler) SetCookieConsent(action *CookieAction) *Crawler {
	app.engine.CookieConsent = action
	return app
}
func (app *Crawler) SetTimeout(timeout time.Duration) *Crawler {
	app.engine.Timeout = timeout * time.Second
	return app
}
func (app *Crawler) DisableJavaScript() *Crawler {
	app.engine.JavaScriptEnabled = false
	return app
}
func (app *Crawler) WaitForDynamicRendering() *Crawler {
	app.engine.WaitForDynamicRendering = true
	return app
}
func (app *Crawler) SetSleepAfter(sleepAfter int) *Crawler {
	app.engine.SleepAfter = sleepAfter
	return app
}

// Todo: getProxyList should be generate dynamically in future
func (app *Crawler) getProxyList() []Proxy {
	proxyEnv := app.Config.GetString("PROXY_SERVERS")
	if proxyEnv == "" {
		return nil // Return an empty list or handle the absence of proxies as needed
	}

	proxyUrls := strings.Split(proxyEnv, ",")
	var proxies []Proxy
	for _, url := range proxyUrls {
		proxies = append(proxies, Proxy{Server: url})
	}
	return proxies
}
func (app *Crawler) BuildQueryString() string {
	params := url.Values{}

	if app.engine.ProviderOption.JsRender {
		params.Add("js_render", "true")
	}
	if app.engine.ProviderOption.CustomHeaders {
		params.Add("custom_headers", "true")
	}
	if app.engine.ProviderOption.PremiumProxy {
		params.Add("premium_proxy", "true")
	}
	if app.engine.ProviderOption.ProxyCountry != "" {
		params.Add("proxy_country", app.engine.ProviderOption.ProxyCountry)
	}
	if app.engine.ProviderOption.SessionID != 0 {
		params.Add("session_id", fmt.Sprintf("%d", app.engine.ProviderOption.SessionID))
	}
	if app.engine.ProviderOption.Device != "" {
		params.Add("device", app.engine.ProviderOption.Device)
	} else {
		params.Add("device", "desktop")
	}
	if app.engine.ProviderOption.OriginalStatus {
		params.Add("original_status", "true")
	}
	if app.engine.ProviderOption.AllowedStatusCodes != "" {
		params.Add("allowed_status_codes", app.engine.ProviderOption.AllowedStatusCodes)
	}
	if app.engine.ProviderOption.WaitFor != "" {
		params.Add("wait_for", app.engine.ProviderOption.WaitFor)
	}
	if app.engine.ProviderOption.Wait != 0 {
		params.Add("wait", fmt.Sprintf("%d", app.engine.ProviderOption.Wait))
	}
	if app.engine.ProviderOption.BlockResources != "" {
		params.Add("block_resources", app.engine.ProviderOption.BlockResources)
	}
	if app.engine.ProviderOption.JSONResponse {
		params.Add("json_response", "true")
	}
	if app.engine.ProviderOption.CSSExtractor != "" {
		params.Add("css_extractor", app.engine.ProviderOption.CSSExtractor)
	}
	if app.engine.ProviderOption.Autoparse {
		params.Add("autoparse", "true")
	}
	if app.engine.ProviderOption.MarkdownResponse {
		params.Add("markdown_response", "true")
	}
	if app.engine.ProviderOption.Screenshot {
		params.Add("screenshot", "true")
	}
	if app.engine.ProviderOption.ScreenshotFullPage {
		params.Add("screenshot_fullpage", "true")
	}
	if app.engine.ProviderOption.ScreenshotSelector != "" {
		params.Add("screenshot_selector", app.engine.ProviderOption.ScreenshotSelector)
	}
	return params.Encode()
}
