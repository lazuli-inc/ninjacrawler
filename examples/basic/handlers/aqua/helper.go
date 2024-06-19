package aqua

import (
	"github.com/lazuli-inc/ninjacrawler"
	"strings"
)

func productNameHandler(ctx ninjacrawler.CrawlerContext) string {
	productName := ctx.Document.Find("h2.example").Text()
	productName = strings.Trim(productName, " \n")

	return productName
}

func getUrlHandler(ctx ninjacrawler.CrawlerContext) string {
	return ctx.UrlCollection.Url
}

func getProductCategory(ctx ninjacrawler.CrawlerContext) string {
	category := ctx.Document.Find("p.ProductDetail_Section_Headline_Sub").First().Text()
	category = strings.Trim(category, " \n")

	return category
}

func getProductDescription(ctx ninjacrawler.CrawlerContext) string {
	description := ctx.Document.Find("div.ProductDetail_Section_Text_Group").Text()
	return description
}

func getProductAttribute(ctx ninjacrawler.CrawlerContext) []ninjacrawler.AttributeItem {
	attributes := []ninjacrawler.AttributeItem{}
	getExampleAttributeService(ctx, &attributes)
	return attributes
}

func getExampleAttributeService(ctx ninjacrawler.CrawlerContext, attributes *[]ninjacrawler.AttributeItem) {
	item := strings.Trim(ctx.Document.Find(".example p").First().Text(), " \n")
	if len(item) > 0 {
		attribute := ninjacrawler.AttributeItem{
			Key:   "example",
			Value: item,
		}
		*attributes = append(*attributes, attribute)
	}
}
