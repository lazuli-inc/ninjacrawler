package ninjacrawler

import (
	"fmt"
	"github.com/go-rod/rod"
	"github.com/playwright-community/playwright-go"
	"go.mongodb.org/mongo-driver/mongo"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var startTime time.Time

const (
	baseCollection = "sites"
	dbFilterLimit  = 1000
	dbTimeout      = 60 * time.Second
)

type Crawler struct {
	*mongo.Client
	Config                *configService
	Name                  string
	Url                   string
	BaseUrl               string
	pw                    *playwright.Playwright
	pwBrowserCtx          playwright.BrowserContext
	pwPage                playwright.Page
	rdBrowser             *rod.Browser
	rdPage                *rod.Page
	UrlSelectors          []UrlSelector
	ProductDetailSelector ProductDetailSelector
	engine                *Engine
	Logger                *defaultLogger
	httpClient            *http.Client
	isLocalEnv            bool
	isStgEnv              bool
	preference            *AppPreference
	userAgent             string
	CurrentProxy          Proxy
	ReqCount              int32
	CurrentProxyIndex     int32
	CurrentCollection     string
	CurrentUrlCollection  UrlCollection
	CurrentUrl            string
	lastWorkingProxyIndex int32
}

func NewCrawler(name, url string, engines ...Engine) *Crawler {
	// Handle other engine overrides as needed
	config := newConfig()

	crawler := &Crawler{
		Name:              name,
		Url:               url,
		Config:            config,
		CurrentProxy:      Proxy{},
		CurrentProxyIndex: 0,
		ReqCount:          int32(0),
	}

	defaultPreference := getDefaultPreference()
	defaultEngine := getDefaultEngine()
	if len(engines) > 0 {
		eng := engines[0]
		crawler.overrideEngineDefaults(&defaultEngine, &eng)
	}
	crawler.engine = &defaultEngine
	logger := newDefaultLogger(crawler, name)
	crawler.Logger = logger
	crawler.Client = crawler.mustGetClient()
	crawler.BaseUrl = crawler.getBaseUrl(url)
	crawler.isLocalEnv = config.GetString("APP_ENV") == "local"
	crawler.isStgEnv = config.GetString("APP_ENV") == "staging"
	crawler.userAgent = config.GetString("USER_AGENT")
	crawler.preference = &defaultPreference
	crawler.lastWorkingProxyIndex = int32(0)
	crawler.engine.ProxyServers = crawler.getProxyServers()
	return crawler
}

func (app *Crawler) Start() {
	defer func() {
		if r := recover(); r != nil {
			app.HandlePanic(r)
		}
	}()
	startTime = time.Now()
	app.Logger.Summary("Crawler started!")

	deleteDB := app.Config.GetBool("DELETE_DB")
	if deleteDB {
		err := app.dropDatabase()
		if err != nil {
			return
		}
	}
	app.newSite()
	app.toggleClient()
}

func (app *Crawler) toggleClient() {
	if *app.engine.IsDynamic {
		pw, err := app.GetPlaywright()
		if err != nil {
			app.Logger.Debug("failed to initialize playwright: %v\n", err)
			app.Logger.Fatal("failed to initialize playwright: %v\n", err)
			return // exit if playwright initialization fails
		}
		app.pw = pw
	} else {
		app.httpClient = app.GetHttpClient()
	}
}

func (app *Crawler) Stop() {
	defer func() {
		if r := recover(); r != nil {
			app.HandlePanic(r)
		}
	}()
	if app.pw != nil {
		app.pw.Stop()
	}
	if app.httpClient != nil {
		app.httpClient.CloseIdleConnections()
	}
	if app.Client != nil {
		app.closeClient()
	}
	// upload logs
	uploadLogs := app.Config.GetBool("UPLOAD_LOGS")
	if uploadLogs {
		app.UploadLogs()
	}

	if *app.engine.StoreHtml {
		app.UploadRawHtml()
	}
	duration := time.Since(startTime)
	app.Logger.Summary("Crawler completed!")
	app.Logger.Summary("Crawling duration %v", duration)

	// stop the crawler after successful crawl
	err := app.StopCrawler()
	if err != nil {
		app.Logger.Debug("Crawler Stop Failed")
	}
}

func (app *Crawler) openBrowsers(proxy Proxy) {
	var err error
	if *app.engine.IsDynamic {
		if *app.engine.Adapter == PlayWrightEngine {
			app.pwBrowserCtx, err = app.GetBrowser(app.pw, app.engine.BrowserType, proxy)
		}
		if *app.engine.Adapter == RodEngine {
			app.rdBrowser, err = app.GetRodBrowser(proxy)
		}
	} else {
		app.httpClient = app.GetHttpClient()
	}
	if err != nil {
		app.Logger.Fatal(err.Error())
	}

}
func (app *Crawler) closeBrowsers() {
	if *app.engine.IsDynamic {
		if app.pwBrowserCtx != nil {
			app.pwBrowserCtx.Close()
		}
		if app.rdBrowser != nil {
			app.rdBrowser.Close()
		}
	} else {
		if app.httpClient != nil {
			app.httpClient.CloseIdleConnections()
		}
	}

}

func (app *Crawler) openPages() {
	var err error
	if *app.engine.IsDynamic {
		if *app.engine.Adapter == PlayWrightEngine {
			app.pwPage, err = app.GetPage(app.pwBrowserCtx)
		}
		if *app.engine.Adapter == RodEngine {
			app.rdPage, err = app.GetRodPage(app.rdBrowser)
		}
	}
	if err != nil {
		app.Logger.Fatal(err.Error())
	}

}

func (app *Crawler) closePages() {
	if app.pwPage != nil {
		app.pwPage.Close()
	}
	if app.rdPage != nil {
		app.rdPage.Close()
	}
}

func (app *Crawler) UploadLogs() {
	app.Logger.Info("Uploading logs...")
	total := 0
	storagePath := fmt.Sprintf("storage/logs/%s", app.Name)
	err := filepath.Walk(storagePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			app.Logger.Error("Error accessing path %s: %v", path, err)
			return err
		}

		if !info.IsDir() {
			relativePath := strings.TrimPrefix(path, storagePath+"/")
			uploadToBucket(app, path, fmt.Sprintf("logs/%s", relativePath))
			total++
		}

		return nil
	})

	app.Logger.Info("Total %d File uploaded to bucket successfully", total)

	if err != nil {
		app.Logger.Error("Error walking through storage directory: %v", err)
	}
}
func (app *Crawler) UploadRawHtml() {
	app.Logger.Info("Uploading raw html...")
	total := 0
	storagePath := fmt.Sprintf("storage/raw_html/%s", app.Name)
	err := filepath.Walk(storagePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			app.Logger.Error("Error accessing path %s: %v", path, err)
			return err
		}

		if !info.IsDir() {
			relativePath := strings.TrimPrefix(path, storagePath+"/")
			uploadToBucket(app, path, fmt.Sprintf("raw_html/%s", relativePath))
			total++
		}

		return nil
	})

	app.Logger.Info("Total %d File uploaded to bucket successfully", total)

	if err != nil {
		app.Logger.Error("Error walking through storage directory: %v", err)
	}
}

func (app *Crawler) GetBaseCollection() string {
	return baseCollection
}

func (app *Crawler) SetPreference(preference AppPreference) *Crawler {

	defaultPreference := getDefaultPreference()

	overridePreferenceDefaults(&defaultPreference, &preference)
	app.preference = &defaultPreference
	return app
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
func (app *Crawler) AutoHandle(configs []ProcessorConfig) {
	defer app.Stop() // Ensure Stop is called after handlers
	app.Start()

	app.CrawlUrls(configs)
}
func getDefaultPreference() AppPreference {
	return AppPreference{
		ExcludeUniqueUrlEntities: []string{},
	}
}
func overridePreferenceDefaults(defaultPreference *AppPreference, preference *AppPreference) {
	if len(preference.ExcludeUniqueUrlEntities) > 0 {
		defaultPreference.ExcludeUniqueUrlEntities = preference.ExcludeUniqueUrlEntities
	}
}

func getDefaultEngine() Engine {
	return Engine{
		BrowserType:             "chromium",
		Provider:                "http",
		ConcurrentLimit:         1,
		IsDynamic:               Bool(false),
		WaitForDynamicRendering: false,
		DevCrawlLimit:           0,
		StgCrawlLimit:           0,
		BlockResources:          false,
		JavaScriptEnabled:       true,
		BlockedURLs: []string{
			"www.googletagmanager.com",
			"google.com",
			"googleapis.com",
			"gstatic.com",
		},
		BoostCrawling:          false,
		CookieConsent:          nil,
		Timeout:                time.Duration(30) * time.Second,
		SleepAfter:             1000,
		MaxRetryAttempts:       3,
		ForceInstallPlaywright: false,
		Args:                   []string{},
		ProviderOption: ProviderQueryOption{
			JsRender:             false,
			UsePremiumProxyRetry: false,
			OriginalStatus:       true,
			CustomHeaders:        true,
		},
		SleepDuration:      10,
		RetrySleepDuration: 0, //30min
		CrawlTimeout:       999999,
		WaitForSelector:    nil,
		ProxyStrategy:      ProxyStrategyConcurrency,
		ErrorCodes: []int{
			403,
		},
		IgnoreRetryOnValidation: Bool(false),
		StoreHtml:               Bool(false),
		SendHtmlToBigquery:      Bool(false),
		Adapter:                 String(PlayWrightEngine),
	}
}

func (app *Crawler) overrideEngineDefaults(defaultEngine *Engine, eng *Engine) {
	if eng.BrowserType != "" {
		defaultEngine.BrowserType = eng.BrowserType
	}
	if eng.Provider != "" {
		defaultEngine.Provider = eng.Provider
	}
	if eng.ConcurrentLimit > 0 {
		defaultEngine.ConcurrentLimit = eng.ConcurrentLimit
	}
	if eng.IsDynamic != nil {
		defaultEngine.IsDynamic = eng.IsDynamic
	}
	if eng.WaitForDynamicRendering {
		defaultEngine.WaitForDynamicRendering = eng.WaitForDynamicRendering
	}
	if eng.DevCrawlLimit > 0 {
		defaultEngine.DevCrawlLimit = eng.DevCrawlLimit
	}
	if eng.StgCrawlLimit > 0 {
		defaultEngine.StgCrawlLimit = eng.StgCrawlLimit
	}
	if eng.BlockResources {
		defaultEngine.BlockResources = eng.BlockResources
	}
	if eng.JavaScriptEnabled {
		defaultEngine.JavaScriptEnabled = eng.JavaScriptEnabled
	}
	if eng.BoostCrawling {
		//defaultEngine.BoostCrawling = eng.BoostCrawling
		//defaultEngine.ProxyServers = app.getProxyList()
	}
	//if len(eng.ProxyServers) > 0 {
	//	config := newConfig()
	//	zenrowsApiKey := config.EnvString("ZENROWS_API_KEY")
	//	for _, proxy := range eng.ProxyServers {
	//		if proxy.Server == ZENROWS {
	//			proxy.Server = fmt.Sprintf("http://%s:@proxy.zenrows.com:8001", zenrowsApiKey)
	//		}
	//		defaultEngine.ProxyServers = append(defaultEngine.ProxyServers, proxy)
	//	}
	//}
	if eng.CookieConsent != nil {
		defaultEngine.CookieConsent = eng.CookieConsent
	}
	if eng.Timeout > 0 {
		defaultEngine.Timeout = time.Duration(eng.Timeout) * time.Second
	}
	if eng.SleepAfter > 0 {
		defaultEngine.SleepAfter = eng.SleepAfter
	}
	if eng.MaxRetryAttempts > 0 {
		defaultEngine.MaxRetryAttempts = eng.MaxRetryAttempts
	}
	if eng.ForceInstallPlaywright {
		defaultEngine.ForceInstallPlaywright = eng.ForceInstallPlaywright
	}
	if len(eng.Args) > 0 {
		defaultEngine.Args = eng.Args
	}

	if eng.ProviderOption.JsRender {
		defaultEngine.ProviderOption.JsRender = eng.ProviderOption.JsRender
	}

	if eng.ProviderOption.CustomHeaders {
		defaultEngine.ProviderOption.CustomHeaders = eng.ProviderOption.CustomHeaders
	}

	if eng.ProviderOption.PremiumProxy {
		defaultEngine.ProviderOption.PremiumProxy = eng.ProviderOption.PremiumProxy
	}

	if eng.ProviderOption.ProxyCountry != "" {
		defaultEngine.ProviderOption.ProxyCountry = eng.ProviderOption.ProxyCountry
	}

	if eng.ProviderOption.SessionID != 0 {
		defaultEngine.ProviderOption.SessionID = eng.ProviderOption.SessionID
	}

	if eng.ProviderOption.Device != "" {
		defaultEngine.ProviderOption.Device = eng.ProviderOption.Device
	}

	if eng.ProviderOption.OriginalStatus {
		defaultEngine.ProviderOption.OriginalStatus = eng.ProviderOption.OriginalStatus
	}

	if eng.ProviderOption.AllowedStatusCodes != "" {
		defaultEngine.ProviderOption.AllowedStatusCodes = eng.ProviderOption.AllowedStatusCodes
	}

	if eng.ProviderOption.WaitFor != "" {
		defaultEngine.ProviderOption.WaitFor = eng.ProviderOption.WaitFor
	}

	if eng.ProviderOption.Wait != 0 {
		defaultEngine.ProviderOption.Wait = eng.ProviderOption.Wait
	}

	if eng.ProviderOption.BlockResources != "" {
		defaultEngine.ProviderOption.BlockResources = eng.ProviderOption.BlockResources
	}

	if eng.ProviderOption.JSONResponse {
		defaultEngine.ProviderOption.JSONResponse = eng.ProviderOption.JSONResponse
	}

	if eng.ProviderOption.CSSExtractor != "" {
		defaultEngine.ProviderOption.CSSExtractor = eng.ProviderOption.CSSExtractor
	}

	if eng.ProviderOption.Autoparse {
		defaultEngine.ProviderOption.Autoparse = eng.ProviderOption.Autoparse
	}

	if eng.ProviderOption.MarkdownResponse {
		defaultEngine.ProviderOption.MarkdownResponse = eng.ProviderOption.MarkdownResponse
	}

	if eng.ProviderOption.Screenshot {
		defaultEngine.ProviderOption.Screenshot = eng.ProviderOption.Screenshot
	}

	if eng.ProviderOption.ScreenshotFullPage {
		defaultEngine.ProviderOption.ScreenshotFullPage = eng.ProviderOption.ScreenshotFullPage
	}

	if eng.ProviderOption.ScreenshotSelector != "" {
		defaultEngine.ProviderOption.ScreenshotSelector = eng.ProviderOption.ScreenshotSelector
	}

	if eng.ProviderOption.UsePremiumProxyRetry {
		defaultEngine.ProviderOption.UsePremiumProxyRetry = eng.ProviderOption.UsePremiumProxyRetry
	}
	defaultEngine.BlockedURLs = append(defaultEngine.BlockedURLs, eng.BlockedURLs...)

	if eng.SleepDuration > 0 {
		defaultEngine.SleepDuration = eng.SleepDuration
	}
	if eng.RetrySleepDuration > 0 {
		defaultEngine.RetrySleepDuration = eng.RetrySleepDuration
	}
	if eng.CrawlTimeout > 0 {
		defaultEngine.CrawlTimeout = eng.CrawlTimeout
	}
	if eng.WaitForSelector != nil {
		defaultEngine.WaitForSelector = eng.WaitForSelector
	}
	if eng.ProxyStrategy != "" {
		defaultEngine.ProxyStrategy = eng.ProxyStrategy
	}
	if eng.ErrorCodes != nil && len(eng.ErrorCodes) > 0 {
		defaultEngine.ErrorCodes = eng.ErrorCodes

	}
	if eng.IgnoreRetryOnValidation != nil {
		defaultEngine.IgnoreRetryOnValidation = eng.IgnoreRetryOnValidation
	}
	if eng.StoreHtml != nil {
		defaultEngine.StoreHtml = eng.StoreHtml
	}
	if eng.SendHtmlToBigquery != nil {
		defaultEngine.SendHtmlToBigquery = eng.SendHtmlToBigquery
	}
	if eng.Adapter != nil {
		defaultEngine.Adapter = eng.Adapter
	}
}

func (app *Crawler) getProxyServers() []Proxy {
	proxies := app.Config.GetString("PROXY_SERVERS")
	proxyUsername := app.Config.GetString("PROXY_USERNAME")
	proxyPassword := app.Config.GetString("PROXY_PASSWORD")
	var proxyServers []Proxy

	if len(proxies) > 0 {
		for _, p := range strings.Split(proxies, ",") {
			proxyServers = append(proxyServers, Proxy{
				Server:   p,
				Username: proxyUsername,
				Password: proxyPassword,
			})
		}
	}
	return proxyServers
}
func (app *Crawler) isCpuUsageHigh() bool {
	//usage, err := cpu.Percent(0, false)
	//if err != nil {
	//	app.Logger.Error("Error retrieving CPU usage: %v", err)
	//	return false
	//}
	//
	//// If CPU usage is over 90%, return true
	//if len(usage) > 0 && usage[0] > 90 {
	//	return true
	//}
	return false
}

// isRamUsageHigh checks if RAM usage exceeds 90%
func (app *Crawler) isRamUsageHigh() bool {
	//v, err := mem.VirtualMemory()
	//if err != nil {
	//	app.Logger.Error("Error retrieving RAM usage: %v", err)
	//	return false
	//}

	// If RAM usage is over 90%, return true
	//if v.UsedPercent > 90 {
	//	return true
	//}
	return false
}
