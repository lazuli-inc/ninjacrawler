package main

import (
	"github.com/lazuli-inc/ninjacrawler"
	aqua2 "github.com/lazuli-inc/ninjacrawler/examples/basic/handlers/aqua"
	markt2 "github.com/lazuli-inc/ninjacrawler/examples/basic/handlers/markt"
	sandvik2 "github.com/lazuli-inc/ninjacrawler/examples/basic/handlers/sandvik"
)

func main() {

	// simple
	ninjacrawler.NewCrawler("aqua", "https://aqua-has.com", ninjacrawler.Engine{
		IsDynamic:       false,
		DevCrawlLimit:   1,
		ConcurrentLimit: 1,
		BlockResources:  true,
		BoostCrawling:   true,
	}).Handle(ninjacrawler.Handler{
		UrlHandler:     aqua2.UrlHandler,
		ProductHandler: aqua2.ProductHandler,
	})
	// medium complex
	ninjacrawler.NewCrawler("markt", "https://markt-mall.jp", ninjacrawler.Engine{
		BrowserType:     "chromium",
		ConcurrentLimit: 1,
		DevCrawlLimit:   5,
		IsDynamic:       true,
		BlockResources:  true,
	}).Handle(ninjacrawler.Handler{
		UrlHandler:     markt2.UrlHandler,
		ProductHandler: markt2.ProductHandler,
	})
	// medium complex
	ninjacrawler.NewCrawler("sandvik", "https://www.sandvik.coromant.com/ja-jp/tools", ninjacrawler.Engine{
		IsDynamic:       false,
		DevCrawlLimit:   1,
		ConcurrentLimit: 1,
		CookieConsent: &ninjacrawler.CookieAction{
			ButtonText:                  "Accept Cookies",
			MustHaveSelectorAfterAction: "body .page-container",
		},
	}).Handle(ninjacrawler.Handler{
		UrlHandler:     sandvik2.UrlHandler,
		ProductHandler: sandvik2.ProductHandler,
	})
}
