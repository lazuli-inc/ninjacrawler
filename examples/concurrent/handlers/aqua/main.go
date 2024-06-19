package aqua

import "github.com/lazuli-inc/ninjacrawler"

func Crawler() ninjacrawler.CrawlerConfig {
	return ninjacrawler.CrawlerConfig{
		Name: "aqua",
		URL:  "https://aqua-has.com",
		Engine: ninjacrawler.Engine{
			IsDynamic:       false,
			DevCrawlLimit:   1,
			ConcurrentLimit: 1,
			BlockResources:  true,
		},
		Handler: ninjacrawler.Handler{
			UrlHandler:     UrlHandler,
			ProductHandler: ProductHandler,
		},
	}
}
