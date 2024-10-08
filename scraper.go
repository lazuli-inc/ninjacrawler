package ninjacrawler

import (
	"github.com/PuerkitoBio/goquery"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

type Map map[string]interface{}

// Get retrieves a nested value from the map using dot-separated path

// Get retrieves a nested value from the map using a path with support for array indexing
func (m Map) Get(path string) interface{} {
	keys := parseKeys(path)
	var result interface{} = m
	for _, key := range keys {
		switch val := result.(type) {
		case Map:
			result = val[key]
		case map[string]interface{}:
			result = val[key]
		case []interface{}:
			index, err := strconv.Atoi(key)
			if err != nil || index < 0 || index >= len(val) {
				return nil
			}
			result = val[index]
		default:
			return nil
		}
	}
	return result
}

func parseKeys(path string) []string {
	//path = strings.ReplaceAll(path, "]", "")
	parts := strings.Split(path, ".")
	keys := make([]string, len(parts))
	for i, part := range parts {
		keys[i] = part
	}
	return keys
}

func (ctx *CrawlerContext) scrapData(processor interface{}) *ProductDetail {
	app := ctx.App
	document := ctx.Document
	productDetail := &ProductDetail{}
	productDetailSelector := reflect.ValueOf(processor)

	for i := 0; i < productDetailSelector.NumField(); i++ {
		fieldValue := productDetailSelector.Field(i)
		fieldType := productDetailSelector.Type().Field(i)
		fieldName := fieldType.Name

		switch v := fieldValue.Interface().(type) {
		case string:
			reflect.ValueOf(productDetail).Elem().FieldByName(fieldName).SetString(v)
		case []string:
			reflect.ValueOf(productDetail).Elem().FieldByName(fieldName).Set(reflect.ValueOf(v))
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
			result := handleMultiSelectors(app, document, selectors)

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

func (ctx *CrawlerContext) handleProductDetailApi(processor interface{}) *ProductDetail {
	app := ctx.App

	productDetail := &ProductDetail{}
	productDetailSelector := reflect.ValueOf(processor)

	for i := 0; i < productDetailSelector.NumField(); i++ {
		fieldValue := productDetailSelector.Field(i)
		fieldType := productDetailSelector.Type().Field(i)
		fieldName := fieldType.Name

		switch v := fieldValue.Interface().(type) {
		case string:
			property := fieldValue.Interface().(string)
			result := ctx.ApiResponse.Get(property)
			if result != nil {
				reflect.ValueOf(productDetail).Elem().FieldByName(fieldName).Set(reflect.ValueOf(result))
			}
		case func(CrawlerContext) []AttributeItem:
			result := fieldValue.Interface().(func(CrawlerContext) []AttributeItem)(*ctx)
			reflect.ValueOf(productDetail).Elem().FieldByName(fieldName).Set(reflect.ValueOf(result))
		case func(CrawlerContext) []string:
			result := fieldValue.Interface().(func(CrawlerContext) []string)(*ctx)
			reflect.ValueOf(productDetail).Elem().FieldByName(fieldName).Set(reflect.ValueOf(result))
		case func(CrawlerContext) string:
			result := fieldValue.Interface().(func(CrawlerContext) string)(*ctx)
			reflect.ValueOf(productDetail).Elem().FieldByName(fieldName).SetString(result)
		default:
			app.Logger.Error("Invalid %s CrawlerContext: %T", fieldName, v)
		}
	}

	return productDetail
}

func handleSingleSelector(document *goquery.Document, selector *SingleSelector) interface{} {
	txt := document.Find(selector.Selector).Text()
	// Handle provided regexps and general cleanup in a single loop
	for _, reStr := range selector.Regexp {
		re := regexp.MustCompile(reStr)
		txt = re.ReplaceAllString(txt, "")
	}

	return txt
}

func handleMultiSelectors(app *Crawler, document *goquery.Document, selectors *MultiSelectors) []interface{} {
	items := []string{}
	itemSet := make(map[string]struct{})

	// Helper function to append images if the specified attribute exists
	appendImages := func(selection *goquery.Selection, attr string) {
		selection.Each(func(i int, s *goquery.Selection) {
			if url, ok := s.Attr(attr); ok {
				fullUrl := app.GetFullUrl(url)

				// Check if the URL contains any excluded strings
				excluded := false
				for _, exclude := range selectors.ExcludeString {
					if strings.Contains(fullUrl, exclude) {
						excluded = true
						break
					}
				}
				if excluded {
					return
				}

				// Add to items if unique or uniqueness is not enforced
				if selectors.IsUnique {
					if _, exists := itemSet[fullUrl]; !exists {
						itemSet[fullUrl] = struct{}{}
						items = append(items, fullUrl)
					}
				} else {
					items = append(items, fullUrl)
				}
			}
		})
	}

	// Process each selector in the array
	for _, selector := range selectors.Selectors {
		appendImages(document.Find(selector.Query), selector.Attr)
	}

	// Convert items to []interface{}
	var result []interface{}
	for _, item := range items {
		result = append(result, item)
	}

	return result
}
