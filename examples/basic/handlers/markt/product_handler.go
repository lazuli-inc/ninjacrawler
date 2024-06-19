package markt

import (
	"github.com/lazuli-inc/ninjacrawler"
	"github.com/lazuli-inc/ninjacrawler/examples/concurrent/constant"
	"strings"
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
				{Query: "img#image-item", Attr: "src"},
				{Query: "section.ProductDetail_Section_Function a", Attr: "href"},
				{Query: "section.ProductDetail_Section_Spec img", Attr: "src"},
			},
		},
		ProductCodes: productCodeHandler,
		Maker:        "",
		Brand:        "",
		ProductName:  productNameHandler,
		Category:     "",
		Description:  "",
	}
	crawler.Collection(constant.ProductDetails).CrawlPageDetail(constant.Products)
}
func productCodeHandler(ctx ninjacrawler.CrawlerContext) []string {
	urlParts := strings.Split(strings.Trim(ctx.UrlCollection.Url, "/"), "/")
	return []string{urlParts[len(urlParts)-1]}
}

func productNameHandler(ctx ninjacrawler.CrawlerContext) string {
	return strings.Trim(ctx.Document.Find("h2.ProductInfo_Head_Main_ProductName").Text(), " \n")
}

func getUrlHandler(ctx ninjacrawler.CrawlerContext) string {
	return ctx.UrlCollection.Url
}
