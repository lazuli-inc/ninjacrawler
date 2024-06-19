package kyocera

import "github.com/lazuli-inc/ninjacrawler"

func Crawler() ninjacrawler.CrawlerConfig {
	return ninjacrawler.CrawlerConfig{
		Name: "kyocera",
		URL:  "https://www.kyocera.co.jp/prdct/tool/category/product",
		Engine: ninjacrawler.Engine{
			//BoostCrawling:  true,
			BlockResources:  true,
			DevCrawlLimit:   1,
			ConcurrentLimit: 1,
			BlockedURLs:     []string{"syncsearch.jp"},
		},
		Handler: ninjacrawler.Handler{
			UrlHandler:     UrlHandler,
			ProductHandler: ProductHandler,
		},
	}
}
