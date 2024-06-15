package kyocera

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/lazuli-inc/ninjacrawler"
)

func ProductHandler(crawler *ninjacrawler.Crawler) {
	crawler.ProductDetailSelector = ninjacrawler.ProductDetailSelector{
		Jan: "",
		PageTitle: &ninjacrawler.SingleSelector{
			Selector: "title",
		},
		Url: GetUrlHandler,
		Images: &ninjacrawler.MultiSelectors{
			Selectors: []ninjacrawler.Selector{
				{Query: ".details .intro .image img", Attr: "src"},
			},
		},
		ProductCodes: func(app ninjacrawler.Crawler, document *goquery.Document, urlCollection ninjacrawler.UrlCollection) []string {
			return []string{}
		},
		Maker:       "",
		Brand:       "",
		ProductName: ProductNameHandler,
		Category:    GetProductCategory,
		Description: GetProductDescription,
		Reviews: func(app ninjacrawler.Crawler, document *goquery.Document, urlCollection ninjacrawler.UrlCollection) []string {
			return []string{}
		},
		ItemTypes: func(app ninjacrawler.Crawler, document *goquery.Document, urlCollection ninjacrawler.UrlCollection) []string {
			return []string{}
		},
		ItemSizes: func(app ninjacrawler.Crawler, document *goquery.Document, urlCollection ninjacrawler.UrlCollection) []string {
			return []string{}
		},
		ItemWeights: func(app ninjacrawler.Crawler, document *goquery.Document, urlCollection ninjacrawler.UrlCollection) []string {
			return []string{}
		},
		SingleItemSize:   "",
		SingleItemWeight: "",
		NumOfItems:       "",
		ListPrice:        "",
		SellingPrice:     "",
		Attributes:       GetProductAttribute,
	}
	crawler.Collection(constant.ProductDetails).CrawlPageDetail(constant.Products)
}
