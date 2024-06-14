package ninjacrawler

import (
	"github.com/playwright-community/playwright-go"
	"sync"
	"time"
)

var Once sync.Once
var app *Crawler
var startTime time.Time

const (
	baseCollection = "sites"
)

type Crawler struct {
	*client
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
	startTime = time.Now()
	logger := newDefaultLogger(name)
	defer func() {
		if r := recover(); r != nil {
			logger.Info("Recovered in NewCrawler: %v\n", r)
		}
	}()
	logger.Info("Program started! 🚀")

	defaultEngine := getDefaultEngine()
	if len(engines) > 0 {
		eng := engines[0]
		overrideEngineDefaults(&defaultEngine, &eng)
	}

	app = &Crawler{
		Name:       name,
		Url:        url,
		BaseUrl:    getBaseUrl(url),
		collection: baseCollection, // use baseCollection directly
		engine:     &defaultEngine,
		Logger:     logger,
		Config:     newConfig(),
	}
	app.Start()

	return app
}

func (a *Crawler) Start() {
	defer func() {
		if r := recover(); r != nil {
			a.Logger.Error("Recovered in Start: %v", r)
		}
	}()
	client := connectDB()
	client.newSite()
	pw, err := GetPlaywright()
	if err != nil {
		a.Logger.Fatal("failed to initialize playwright: %v\n", err)
		return // exit if playwright initialization fails
	}

	a.client = client
	a.pw = pw
}

func (a *Crawler) Stop() {
	defer func() {
		if r := recover(); r != nil {
			a.Logger.Error("Recovered in Stop: %v", r)
		}
	}()
	if a.pw != nil {
		a.pw.Stop()
	}
	if a.client != nil {
		a.client.close()
	}
	duration := time.Since(startTime)
	a.Logger.Info("Program stopped in ⚡ %v", duration)
}

func (a *Crawler) Collection(collection string) *Engine {
	a.collection = collection
	return a.engine
}

func (a *Crawler) GetCollection() string {
	return a.collection
}

func (a *Crawler) GetBaseCollection() string {
	return baseCollection
}

type Handler struct {
	UrlHandler     func(c *Crawler)
	ProductHandler func(c *Crawler)
}

func (a *Crawler) Handle(handler Handler) {
	defer a.Stop() // Ensure Stop is called after handlers

	if handler.UrlHandler != nil {
		handler.UrlHandler(a)
	}
	if handler.ProductHandler != nil {
		handler.ProductHandler(a)
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
	defaultEngine.BlockedURLs = append(defaultEngine.BlockedURLs, eng.BlockedURLs...)
}
