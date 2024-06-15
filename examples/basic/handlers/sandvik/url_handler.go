package sandvik

import (
	"github.com/lazuli-inc/ninjacrawler"
	"github.com/lazuli-inc/ninjacrawler/examples/basic/constant"
)

func UrlHandler(crawler *ninjacrawler.Crawler) {
	categorySelector := ninjacrawler.UrlSelector{
		Selector:     ".row.mb-6.ng-star-inserted .col-md-6.col-lg-3.mb-3.mb-md-4.ng-star-inserted",
		SingleResult: false,
		FindSelector: "a",
		Attr:         "href",
	}
	subCategorySelector := ninjacrawler.UrlSelector{
		Selector:     ".col-md-6.col-lg-3.mb-2.mb-md-4.ng-star-inserted",
		SingleResult: false,
		FindSelector: "a",
		Attr:         "href",
	}
	seriesSelector := ninjacrawler.UrlSelector{
		Selector:     ".row.mb-6.ng-star-inserted .col-md-6.col-lg-3.mb-2.mb-md-4 .ng-star-inserted",
		SingleResult: false,
		FindSelector: "a",
		Attr:         "href",
	}
	crawler.Collection(constant.Categories).CrawlUrls(crawler.GetBaseCollection(), categorySelector)
	crawler.Collection(constant.SubCategories).CrawlUrls(constant.Categories, subCategorySelector)
	crawler.Collection(constant.Series).CrawlUrls(constant.SubCategories, seriesSelector)

}
