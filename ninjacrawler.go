package ninjacrawler

import (
	"sync"
)

// CrawlerConfig holds the configuration for a single crawler
type CrawlerConfig struct {
	Name    string
	URL     string
	Engine  Engine
	Handler Handler
}

type NinjaCrawler struct {
	Config []CrawlerConfig
}

func NewNinjaCrawler() *NinjaCrawler {
	return &NinjaCrawler{}
}

func (ninja *NinjaCrawler) AddSite(config CrawlerConfig) *NinjaCrawler {
	ninja.Config = append(ninja.Config, config)
	return ninja
}

func (ninja *NinjaCrawler) Start() {
	var wg sync.WaitGroup
	wg.Add(len(ninja.Config))

	for _, config := range ninja.Config {
		cfg := config // Capture config variable for each goroutine
		go func(cfg CrawlerConfig) {
			defer wg.Done()
			NewCrawler(cfg.Name, cfg.URL, cfg.Engine).Handle(cfg.Handler)
		}(cfg)
	}

	wg.Wait()
}
