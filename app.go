package ninjacrawler

import (
	"github.com/playwright-community/playwright-go"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

var startTime time.Time

const (
	baseCollection = "sites"
)

type Crawler struct {
	*mongo.Client
	Config                *configService
	Name                  string
	Url                   string
	BaseUrl               string
	pw                    *playwright.Playwright
	collection            string
	UrlSelectors          []UrlSelector
	ProductDetailSelector ProductDetailSelector
	engine                *Engine
	Logger                *defaultLogger
}

func NewCrawler(name, url string, engines ...Engine) *Crawler {

	defaultEngine := getDefaultEngine()
	if len(engines) > 0 {
		eng := engines[0]
		overrideEngineDefaults(&defaultEngine, &eng)
	}
	// Handle other engine overrides as needed
	config := newConfig()

	crawler := &Crawler{
		Name:       name,
		Url:        url,
		collection: baseCollection,
		engine:     &defaultEngine,
		Config:     config,
	}

	logger := newDefaultLogger(crawler, name)
	logger.Info("Crawler Started! 🚀")
	crawler.Logger = logger
	crawler.Client = crawler.mustGetClient()
	crawler.BaseUrl = crawler.getBaseUrl(url)
	return crawler
}

func (app *Crawler) Start() {
	defer func() {
		if r := recover(); r != nil {
			app.Logger.Error("Recovered in Start: %v", r)
		}
	}()
	app.newSite()
	pw, err := GetPlaywright()
	if err != nil {
		app.Logger.Fatal("failed to initialize playwright: %v\n", err)
		return // exit if playwright initialization fails
	}
	app.pw = pw
}

func (app *Crawler) Stop() {
	defer func() {
		if r := recover(); r != nil {
			app.Logger.Error("Recovered in Stop: %v", r)
		}
	}()
	if app.pw != nil {
		app.pw.Stop()
	}
	if app.Client != nil {
		app.closeClient()
	}

	duration := time.Since(startTime)
	app.Logger.Info("Crawler stopped in ⚡ %v", duration)
}

func (app *Crawler) Collection(collection string) *Crawler {
	app.collection = collection
	return app
}

func (app *Crawler) GetCollection() string {
	return app.collection
}

func (app *Crawler) GetBaseCollection() string {
	return baseCollection
}

type Handler struct {
	UrlHandler     func(c *Crawler)
	ProductHandler func(c *Crawler)
}

func (app *Crawler) Handle(handler Handler) {
	defer app.Stop() // Ensure Stop is called after handlers
	app.Start()

	if handler.UrlHandler != nil {
		handler.UrlHandler(app)
	}
	if handler.ProductHandler != nil {
		handler.ProductHandler(app)
	}
}

func getDefaultEngine() Engine {
	return Engine{
		BrowserType:     "chromium",
		ConcurrentLimit: 1,
		IsDynamic:       false,
		DevCrawlLimit:   50,
		BlockResources:  false,
		BlockedURLs: []string{
			"www.googletagmanager.com",
			"google.com",
			"googleapis.com",
			"gstatic.com",
		},
		BoostCrawling: false,
		ProxyServers:  []Proxy{},
		CookieConsent: nil,
		Timeout:       30 * 1000, // 30 sec
	}
}

func overrideEngineDefaults(defaultEngine *Engine, eng *Engine) {
	if eng.BrowserType != "" {
		defaultEngine.BrowserType = eng.BrowserType
	}
	if eng.ConcurrentLimit > 0 {
		defaultEngine.ConcurrentLimit = eng.ConcurrentLimit
	}
	if eng.IsDynamic {
		defaultEngine.IsDynamic = eng.IsDynamic
	}
	if eng.DevCrawlLimit > 0 {
		defaultEngine.DevCrawlLimit = eng.DevCrawlLimit
	}
	if eng.BlockResources {
		defaultEngine.BlockResources = eng.BlockResources
	}
	if eng.BoostCrawling {
		defaultEngine.BoostCrawling = eng.BoostCrawling
		defaultEngine.ProxyServers = eng.getProxyList()
	}
	if len(eng.ProxyServers) > 0 {
		defaultEngine.ProxyServers = eng.ProxyServers
	}
	if eng.CookieConsent != nil {
		defaultEngine.CookieConsent = eng.CookieConsent
	}
	if eng.Timeout > 0 {
		defaultEngine.Timeout = eng.Timeout * 1000
	}
	defaultEngine.BlockedURLs = append(defaultEngine.BlockedURLs, eng.BlockedURLs...)
}
