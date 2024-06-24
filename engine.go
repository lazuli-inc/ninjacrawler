package ninjacrawler

import (
	"time"
)

type Engine struct {
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
	app.engine.ProxyServers = app.engine.getProxyList()
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
func (e *Engine) getProxyList() []Proxy {
	var proxies []Proxy
	proxies = append(proxies, Proxy{
		Server: "http://34.146.11.125:3000", // Proxy-server-1
	}, Proxy{
		Server: "http://34.146.155.165:3000", // Proxy-server-2
	}, Proxy{
		Server: "http://34.146.38.231:3000", // Proxy-server-3
	})
	return proxies
}
