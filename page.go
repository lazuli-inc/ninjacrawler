package ninjacrawler

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/playwright-community/playwright-go"
	"reflect"
)

type CrawlerContext struct {
	App           *Crawler
	Document      *goquery.Document
	UrlCollection UrlCollection
	Page          playwright.Page
}

func (ctx *CrawlerContext) handleProductDetail() *ProductDetail {
	app := ctx.App
	document := ctx.Document

	productDetail := &ProductDetail{}
	productDetailSelector := reflect.ValueOf(app.ProductDetailSelector)

	for i := 0; i < productDetailSelector.NumField(); i++ {
		fieldValue := productDetailSelector.Field(i)
		fieldType := productDetailSelector.Type().Field(i)
		fieldName := fieldType.Name

		switch v := fieldValue.Interface().(type) {
		case string:
		case func(CrawlerContext) []AttributeItem:
			result := fieldValue.Interface().(func(CrawlerContext) []AttributeItem)(*ctx)
			reflect.ValueOf(productDetail).Elem().FieldByName(fieldName).Set(reflect.ValueOf(result))
		case func(CrawlerContext) []string:
			result := fieldValue.Interface().(func(CrawlerContext) []string)(*ctx)
			reflect.ValueOf(productDetail).Elem().FieldByName(fieldName).Set(reflect.ValueOf(result))
		case func(CrawlerContext) string:
			result := fieldValue.Interface().(func(CrawlerContext) string)(*ctx)
			reflect.ValueOf(productDetail).Elem().FieldByName(fieldName).SetString(result)
		case *SingleSelector:
			selector := fieldValue.Interface().(*SingleSelector)
			result := handleSingleSelector(document, selector)
			reflect.ValueOf(productDetail).Elem().FieldByName(fieldName).SetString(result.(string))
		case *MultiSelectors:
			selectors := fieldValue.Interface().(*MultiSelectors)
			result := handleMultiSelectors(document, selectors)

			var stringSlice []string
			for _, v := range result {
				stringSlice = append(stringSlice, v.(string))
			}

			reflect.ValueOf(productDetail).Elem().FieldByName(fieldName).Set(reflect.ValueOf(stringSlice))
		default:
			app.Logger.Error("Invalid %s CrawlerContext: %T", fieldName, v)
		}
	}

	return productDetail
}

func handleSingleSelector(document *goquery.Document, selector *SingleSelector) interface{} {
	txt := document.Find(selector.Selector).Text()
	return txt
}

func handleMultiSelectors(document *goquery.Document, selectors *MultiSelectors) []interface{} {
	var items []interface{}

	// Helper function to append images if the specified attribute exists
	appendImages := func(selection *goquery.Selection, attr string) {
		selection.Each(func(i int, s *goquery.Selection) {
			if url, ok := s.Attr(attr); ok {
				items = append(items, url)
			}
		})
	}

	// Process each selector in the array
	for _, selector := range selectors.Selectors {
		appendImages(document.Find(selector.Query), selector.Attr)
	}

	return items
}
