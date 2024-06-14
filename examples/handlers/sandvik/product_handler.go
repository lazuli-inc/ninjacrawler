package sandvik

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/lazuli-inc/ninjacrawler"
	"github.com/lazuli-inc/ninjacrawler/examples/constant"
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

func productNameHandler(document *goquery.Document, urlCollection ninjacrawler.UrlCollection) string {
	return strings.Trim(document.Find(".details .intro h2").First().Text(), " \n")
}

func getUrlHandler(document *goquery.Document, urlCollection ninjacrawler.UrlCollection) string {
	return urlCollection.Url
}
func getProductCategory(document *goquery.Document, urlCollection ninjacrawler.UrlCollection) string {
	categoryItems := make([]string, 0)
	document.Find("ol.st-Breadcrumb_List li.st-Breadcrumb_Item").Each(func(i int, s *goquery.Selection) {
		// Skip the first two items
		if i >= 2 {
			txt := strings.TrimSpace(s.Text())
			categoryItems = append(categoryItems, txt)
		}
	})
	return strings.Join(categoryItems, " > ")
}

func getProductDescription(document *goquery.Document, urlCollection ninjacrawler.UrlCollection) string {

	description := document.Find(".details .intro .text p").Text()
	description = strings.ReplaceAll(description, "\n\n", "\n")

	return description
}
func getProductAttribute(document *goquery.Document, urlCollection ninjacrawler.UrlCollection) []ninjacrawler.AttributeItem {
	attributes := []ninjacrawler.AttributeItem{}

	getCatchCopyAttributeService(document, &attributes)
	getMeritAttributeService(document, &attributes)
	getCatalogAttributeService(document, &attributes)

	return attributes
}

func getCatchCopyAttributeService(document *goquery.Document, attributes *[]ninjacrawler.AttributeItem) {
	// Custom Logic implementation
}

func getMeritAttributeService(document *goquery.Document, attributes *[]ninjacrawler.AttributeItem) {
	key := strings.Trim(document.Find(".merit.clearfix h3").First().Text(), " \n")
	values := strings.Trim(document.Find(".merit.clearfix ul").First().Text(), " \n")

	if len(values) > 0 {
		attribute := ninjacrawler.AttributeItem{
			Key:   key,
			Value: values,
		}
		*attributes = append(*attributes, attribute)
	}
}

func getCatalogAttributeService(document *goquery.Document, attributes *[]ninjacrawler.AttributeItem) {
	// Custom Logic implementation
}
