package ninjacrawler

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
	"github.com/playwright-community/playwright-go"
	"sync"
)

const ZENROWS = "zenrows"

type ProcessorConfig struct {
	Entity           string `json:"entity"`
	OriginCollection string `json:"originCollection"`
	CollectionIndex  *[]string
	Processor        interface{}
	Preference       Preference    `json:"preference"`
	Engine           Engine        `json:"engine"`
	ProcessorType    ProcessorType `json:"processor_type"`
	StateHandler     func(ctx CrawlerContext) Map
}
type ProcessorType struct {
	Handle          *Handle         `json:"handle"`
	UrlSelector     UrlSelector     `json:"url_selector"`
	ElementSelector ElementSelector `json:"element_selector"`
}
type ElementSelector struct {
	Handle   *Handle       `json:"handle"`
	Elements []ElementType `json:"elements"`
}
type ElementType struct {
	Plugin         string         `json:"plugin"`
	SingleSelector SingleSelector `json:"single_selector"`
	MultiSelectors MultiSelectors `json:"multi_selectors"`
	Value          string         `json:"value"`
	ElementID      string         `json:"element_id"`
}
type AppPreference struct {
	ExcludeUniqueUrlEntities []string
	CheckRobotsTxt           *bool
}
type Preference struct {
	DoNotMarkAsComplete   bool
	ValidationRules       []string
	PreHandlers           []func(c PreHandlerContext) error
	ValidationRetryConfig *Engine
}
type PreHandlerContext struct {
	App           *Crawler
	UrlCollection UrlCollection
}
type Handler struct {
	UrlHandler     func(c *Crawler)
	ProductHandler func(c *Crawler)
}
type Proxy struct {
	ID       string
	Server   string
	Username string
	Password string
}
type FormInput struct {
	Key string
	Val string
}
type CookieAction struct {
	Selector                    string
	ButtonText                  string
	MustHaveSelectorAfterAction string
	IsOptional                  bool
	Fields                      []FormInput
}

type CrawlerContext struct {
	App           *Crawler
	Document      *goquery.Document
	UrlCollection UrlCollection
	Page          playwright.Page
	RodPage       *rod.Page
	ApiResponse   Map
	State         Map
}
type NavigationContext struct {
	Document *goquery.Document
	Response interface{}
}

// Struct to hold both results and the UrlCollection
type CrawlResult struct {
	Results       interface{}
	UrlCollection UrlCollection
	Page          playwright.Page
	RodPage       *rod.Page
	Document      *goquery.Document
}
type RequestMetrics struct {
	ReqCount           int32      // Total requests since the crawl started
	HourlyReqCount     int32      // Requests in the last hour
	MinuteReqCount     int32      // Requests in the last minute
	FiveMinuteReqCount int32      // Requests in the last 5 minute
	DayReqCount        int32      // Requests in the last day
	FailedCount        int32      // Track failed requests
	ReportMutex        sync.Mutex // Mutex for thread-safe metric updates
	// Additional fields...
}
