package ninjacrawler

import (
	"strings"
	"time"
)

type Engine struct {
	ForceInstallPlaywright  bool
	Provider                string // http,playwright,zenrows
	ProviderOption          ProviderQueryOption
	BrowserType             string
	ConcurrentLimit         int
	IsDynamic               bool
	DevCrawlLimit           int
	BlockResources          bool
	JavaScriptEnabled       bool
	BlockedURLs             []string
	BoostCrawling           bool
	ProxyServers            []Proxy
	CookieConsent           *CookieAction
	Timeout                 time.Duration
	WaitForDynamicRendering bool
	SleepAfter              int
	MaxRetryAttempts        int
	Args                    []string
	SleepDuration           int
}
type ProviderQueryOption struct {
	JsRender             bool
	UsePremiumProxyRetry bool
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
	app.engine.IsDynamic = isDynamic
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
