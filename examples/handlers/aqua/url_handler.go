package aqua

import (
	"github.com/lazuli-inc/ninjacrawler"
	"github.com/lazuli-inc/ninjacrawler/examples/constant"
)

func UrlHandler(crawler *ninjacrawler.Crawler) {
	categorySelector := ninjacrawler.UrlSelector{
		Selector:     "ul.Header_Navigation_List_Item_Sub_Group_Inner",
		SingleResult: false,
		FindSelector: "a",
		Attr:         "href",
	}
	productSelector := ninjacrawler.UrlSelector{
		Selector:     "div.CategoryTop_Series_Item_Content_List",
		SingleResult: false,
		FindSelector: "a",
		Attr:         "href",
	}
	crawler.Collection(constant.Categories).CrawlUrls(crawler.GetBaseCollection(), categorySelector)
	crawler.Collection(constant.Products).SetCookieConsent(&ninjacrawler.CookieAction{
		ButtonText: "Accept Cookie",
	}).CrawlUrls(constant.Categories, productSelector)

}
