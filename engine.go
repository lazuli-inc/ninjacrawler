package ninjacrawler

import (
	"time"
)

type Proxy struct {
	Server   string
	Username string
	Password string
}
type Engine struct {
	BrowserType     string
	ConcurrentLimit int
	IsDynamic       bool
	DevCrawlLimit   int
	BlockResources  bool
	BlockedURLs     []string
	BoostCrawling   bool
	ProxyServers    []Proxy
	CookieConsent   *CookieAction
	Timeout         time.Duration
}
type FormInput struct {
	Key string
	Val string
}
type CookieAction struct {
	ButtonText                  string
	MustHaveSelectorAfterAction string
	Fields                      []FormInput
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

// Todo: getProxyList should be generate dynamically in future
func (e *Engine) getProxyList() []Proxy {
	var proxies []Proxy
	proxies = append(proxies, Proxy{
		Server: "http://34.146.11.125:3000", // Proxy-server-1
	}, Proxy{
		Server: "http://34.146.155.165:3000", // Proxy-server-2
	}, Proxy{
		Server: "http://34.143.176.68:3000", // Proxy-server-3
	})
	return proxies
}
