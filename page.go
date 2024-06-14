package ninjacrawler

import (
	"github.com/PuerkitoBio/goquery"
	"reflect"
)

func handleProductDetail(document *goquery.Document, urlCollection UrlCollection) *ProductDetail {
	// Create a new ProductDetail struct
	productDetail := &ProductDetail{}

	// Get the reflect.Value of the ProductDetailSelector struct
	productDetailSelector := reflect.ValueOf(app.ProductDetailSelector)

	// Iterate over the fields of ProductDetailSelector
	for i := 0; i < productDetailSelector.NumField(); i++ {
		// Get the field value and type
		fieldValue := productDetailSelector.Field(i)
		fieldType := productDetailSelector.Type().Field(i)

		// Get field name
		fieldName := fieldType.Name

		// Get field value type

		switch v := fieldValue.Interface().(type) {
		case string:
			// String value, do nothing
		case func(*goquery.Document, UrlCollection) []AttributeItem:
			// Call the function and set the result to the corresponding field in ProductDetail
			result := fieldValue.Interface().(func(*goquery.Document, UrlCollection) []AttributeItem)(document, urlCollection)
			reflect.ValueOf(productDetail).Elem().FieldByName(fieldName).Set(reflect.ValueOf(result))
		case func(*goquery.Document, UrlCollection) []string:
			// Call the function and set the result to the corresponding field in ProductDetail
			result := fieldValue.Interface().(func(*goquery.Document, UrlCollection) []string)(document, urlCollection)
			reflect.ValueOf(productDetail).Elem().FieldByName(fieldName).Set(reflect.ValueOf(result))
		case func(*goquery.Document, UrlCollection) string:
			// Call the function and set the result to the corresponding field in ProductDetail
			result := fieldValue.Interface().(func(*goquery.Document, UrlCollection) string)(document, urlCollection)
			reflect.ValueOf(productDetail).Elem().FieldByName(fieldName).SetString(result)
		case *SingleSelector:
			// Handle SingleSelector type
			selector := fieldValue.Interface().(*SingleSelector)
			result := handleSingleSelector(document, selector)
			reflect.ValueOf(productDetail).Elem().FieldByName(fieldName).SetString(result.(string))
		case *MultiSelectors:
			// Handle MultiSelectors type
			selectors := fieldValue.Interface().(*MultiSelectors)
			result := handleMultiSelectors(document, selectors)

			// Convert result from []AttributeItem to []string
			var stringSlice []string
			for _, v := range result {
				stringSlice = append(stringSlice, v.(string))
			}

			reflect.ValueOf(productDetail).Elem().FieldByName(fieldName).Set(reflect.ValueOf(stringSlice))
		default:
			app.Logger.Error("Invalid %s Handler/Selector/Value: %T", fieldName, v)
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
