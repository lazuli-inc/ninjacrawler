package ninjacrawler

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/playwright-community/playwright-go"
)

const ZENROWS = "zenrows"

type ProcessorConfig struct {
	Entity           string `json:"entity"`
	OriginCollection string `json:"originCollection"`
	Processor        interface{}
	Preference       Preference    `json:"preference"`
	Engine           Engine        `json:"engine"`
	ProcessorType    ProcessorType `json:"processor_type"`
}
type ProcessorType struct {
	Handle      *Handle     `json:"handle"`
	UrlSelector UrlSelector `json:"url_selector"`
}
type AppPreference struct {
	ExcludeUniqueUrlEntities []string
}
type Preference struct {
	DoNotMarkAsComplete bool
	ValidationRules     []string
}

type Handler struct {
	UrlHandler     func(c *Crawler)
	ProductHandler func(c *Crawler)
}
type Proxy struct {
	Server   string
	Username string
	Password string
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

type CrawlerContext struct {
	App           *Crawler
	Document      *goquery.Document
	UrlCollection UrlCollection
	Page          playwright.Page
	ApiResponse   Map
}

// Struct to hold both results and the UrlCollection
type CrawlResult struct {
	Results       interface{}
	UrlCollection UrlCollection
	Page          playwright.Page
	Document      *goquery.Document
}
