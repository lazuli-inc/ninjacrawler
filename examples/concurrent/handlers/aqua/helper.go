package aqua

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/lazuli-inc/ninjacrawler"
	"strings"
)

func productNameHandler(app ninjacrawler.Crawler, document *goquery.Document, urlCollection ninjacrawler.UrlCollection) string {
	return strings.Trim(document.Find(".details .intro h2").First().Text(), " \n")
}

func getUrlHandler(app ninjacrawler.Crawler, document *goquery.Document, urlCollection ninjacrawler.UrlCollection) string {
	return urlCollection.Url
}
func getProductCategory(app ninjacrawler.Crawler, document *goquery.Document, urlCollection ninjacrawler.UrlCollection) string {
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

func getProductDescription(app ninjacrawler.Crawler, document *goquery.Document, urlCollection ninjacrawler.UrlCollection) string {

	description := document.Find(".details .intro .text p").Text()
	description = strings.ReplaceAll(description, "\n\n", "\n")

	return description
}
func getProductAttribute(app ninjacrawler.Crawler, document *goquery.Document, urlCollection ninjacrawler.UrlCollection) []ninjacrawler.AttributeItem {
	attributes := []ninjacrawler.AttributeItem{}

	getCatchCopyAttributeService(app, document, &attributes)
	getMeritAttributeService(app, document, &attributes)
	getCatalogAttributeService(app, document, &attributes)

	return attributes
}

func getCatchCopyAttributeService(app ninjacrawler.Crawler, document *goquery.Document, attributes *[]ninjacrawler.AttributeItem) {
	item := strings.Trim(document.Find(".details .intro p.top").First().Text(), " \n")

	if len(item) > 0 {
		attribute := ninjacrawler.AttributeItem{
			Key:   "catch_copy",
			Value: item,
		}
		*attributes = append(*attributes, attribute)
	}
}

func getMeritAttributeService(app ninjacrawler.Crawler, document *goquery.Document, attributes *[]ninjacrawler.AttributeItem) {
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

func getCatalogAttributeService(app ninjacrawler.Crawler, document *goquery.Document, attributes *[]ninjacrawler.AttributeItem) {
	document.Find("#detail ul li").Each(func(i int, s *goquery.Selection) {
		a := s.Find("a")
		key := strings.Trim(a.Text(), " \n")
		img := s.Find("img")
		alt, exist := img.Attr("alt")
		if exist {
			key = alt
		}

		value, exists := a.Attr("href")

		if exists {
			fullUrl := app.GetFullUrl(value)

			attribute := ninjacrawler.AttributeItem{
				Key:   key,
				Value: fullUrl,
			}
			*attributes = append(*attributes, attribute)
		}
	})
}
