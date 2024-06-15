package kyocera

import (
	"github.com/lazuli-inc/ninjacrawler"
	"github.com/lazuli-inc/ninjacrawler/examples/concurrent/constant"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func UrlHandler(crawler *ninjacrawler.Crawler) {
	// Define the common selector parameters
	commonSelectorParams := map[string]string{
		"Selector":     "div.index.clearfix ul.clearfix li",
		"FindSelector": "a",
		"Attr":         "href",
	}

	// Create category selectors with different handlers
	categorySelectors := []struct {
		handler func(ninjacrawler.UrlCollection, string, *goquery.Selection) (string, map[string]interface{})
		target  string
	}{
		{getCategoryHandler("/tool/category/"), constant.Categories},
		{getCategoryHandler("/tool/product/"), constant.Products},
		{getCategoryHandler("/tool/sgs/"), constant.Other},
	}

	// Register category selectors
	for _, cs := range categorySelectors {
		selector := createUrlSelector(commonSelectorParams, cs.handler)
		crawler.Collection(cs.target).CrawlUrls(crawler.GetBaseCollection(), selector)
	}

	// Define the product selector
	productSelector := ninjacrawler.UrlSelector{
		Selector:     "ul.product-list li.product-item,ul.heightLineParent.clearfix li",
		SingleResult: false,
		FindSelector: "a,div dl dt a",
		Attr:         "href",
	}

	// Register product selectors for categories and other
	crawler.Collection(constant.Products).CrawlUrls(constant.Categories, productSelector)
	crawler.Collection(constant.Products).CrawlUrls(constant.Other, productSelector)
}

// createUrlSelector generates a UrlSelector with common parameters and a specific handler
func createUrlSelector(params map[string]string, handler func(ninjacrawler.UrlCollection, string, *goquery.Selection) (string, map[string]interface{})) ninjacrawler.UrlSelector {
	return ninjacrawler.UrlSelector{
		Selector:     params["Selector"],
		SingleResult: false,
		FindSelector: params["FindSelector"],
		Attr:         params["Attr"],
		Handler:      handler,
	}
}

// getCategoryHandler returns a handler function that filters URLs based on the given pattern
func getCategoryHandler(pattern string) func(ninjacrawler.UrlCollection, string, *goquery.Selection) (string, map[string]interface{}) {
	return func(collection ninjacrawler.UrlCollection, fullUrl string, a *goquery.Selection) (string, map[string]interface{}) {
		if strings.Contains(fullUrl, pattern) {
			return fullUrl, nil
		}
		return "", nil
	}
}
