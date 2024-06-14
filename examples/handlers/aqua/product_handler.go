package aqua

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/lazuli-inc/ninjacrawler"
	"github.com/lazuli-inc/ninjacrawler/examples/constant"
)

func ProductHandler(crawler *ninjacrawler.Crawler) {
	crawler.ProductDetailSelector = ninjacrawler.ProductDetailSelector{
		Jan: "",
		PageTitle: &ninjacrawler.SingleSelector{
			Selector: "title",
		},
		Url: getUrlHandler,
		Images: &ninjacrawler.MultiSelectors{
			Selectors: []ninjacrawler.Selector{
				{Query: ".details .intro .image img", Attr: "src"},
			},
		},
		ProductCodes:     func(document *goquery.Document, urlCollection ninjacrawler.UrlCollection) []string { return []string{} },
		Maker:            "",
		Brand:            "",
		ProductName:      productNameHandler,
		Category:         getProductCategory,
		Description:      getProductDescription,
		Reviews:          func(document *goquery.Document, urlCollection ninjacrawler.UrlCollection) []string { return []string{} },
		ItemTypes:        func(document *goquery.Document, urlCollection ninjacrawler.UrlCollection) []string { return []string{} },
		ItemSizes:        func(document *goquery.Document, urlCollection ninjacrawler.UrlCollection) []string { return []string{} },
		ItemWeights:      func(document *goquery.Document, urlCollection ninjacrawler.UrlCollection) []string { return []string{} },
		SingleItemSize:   "",
		SingleItemWeight: "",
		NumOfItems:       "",
		ListPrice:        "",
		SellingPrice:     "",
		Attributes:       getProductAttribute,
	}
	crawler.Collection(constant.ProductDetails).CrawlPageDetail(constant.Products)
}
