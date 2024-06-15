package ninjacrawler

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// ExportProductDetailsToCSV exports product details to CSV files in chunks.
func exportProductDetailsToCSV(crawler *Crawler, collection string, startPage int) {
	fileName := generateCsvFileName(crawler.Name)
	page := startPage

	for {
		products := crawler.GetProductDetailCollections(collection, page)
		if len(products) == 0 {
			break
		}
		err := mustWriteDataToCSV(crawler, fileName, products, page == startPage)
		if err != nil {
			crawler.Logger.Error("Error writing data to CSV: %v", err)
			return
		}
		page++
	}

	fileNameParts := strings.Split(fileName, "/")
	uploadFileName := fileNameParts[len(fileNameParts)-1]
	uploadToBucket(crawler, fileName, uploadFileName)
}

func mustWriteDataToCSV(crawler *Crawler, filename string, products []ProductDetail, isFirstPage bool) error {
	// Ensure the directory exists
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	fileFlag := os.O_WRONLY | os.O_CREATE | os.O_APPEND
	if isFirstPage {
		fileFlag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}

	file, err := os.OpenFile(filename, fileFlag, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if isFirstPage {
		header := []string{
			"jan",
			"page_title",
			"url",
			"images",
			"product_codes",
			"maker",
			"brand",
			"product_name",
			"category",
			"description",
			"reviews",
			"item_types",
			"item_sizes",
			"item_weights",
			"single_item_size",
			"single_item_weight",
			"num_of_items",
			"list_price",
			"selling_price",
			"attributes",
		}
		if err := writer.Write(header); err != nil {
			return fmt.Errorf("failed to write CSV header: %w", err)
		}
	}

	for _, product := range products {
		row, err := convertFieldsToStrings(product)
		if err != nil {
			// Assuming there's a logger defined elsewhere
			crawler.Logger.Error("Error converting product fields to strings:", err)
			continue
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write record to CSV: %w", err)
		}
	}

	return writer.Error()
}
func convertFieldsToStrings(product ProductDetail) ([]string, error) {
	var stringFields []string
	value := reflect.ValueOf(product)

	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		fieldName := strings.ToLower(value.Type().Field(i).Name)
		convertedString := ""

		switch field.Kind() {
		case reflect.String:
			convertedString = field.String()
		case reflect.Slice, reflect.Array, reflect.Map:
			if fieldName == "attributes" {
				jsonData, err := processAttributeColumn(field.Interface())
				if err != nil {
					return nil, fmt.Errorf("error processing attributes: %w", err)
				}
				convertedString = string(jsonData)
			} else {
				jsonData, err := json.Marshal(field.Interface())
				if err != nil {
					return nil, fmt.Errorf("error marshalling field %s: %w", fieldName, err)
				}
				convertedString = processEncodedString(string(jsonData))
			}
		default:
			jsonData, err := json.Marshal(field.Interface())
			if err != nil {
				return nil, fmt.Errorf("error marshalling field %s: %w", fieldName, err)
			}
			convertedString = processEncodedString(string(jsonData))
		}

		stringFields = append(stringFields, convertedString)
	}

	return stringFields, nil
}

func processEncodedString(text string) string {
	replacer := strings.NewReplacer("\\n", "\n", "\\u003e", ">", "\\u0026", "&")
	return replacer.Replace(text)
}

func processAttributeColumn(data interface{}) ([]byte, error) {
	attributes, ok := data.([]AttributeItem)
	if !ok {
		return nil, fmt.Errorf("invalid attribute data type")
	}

	attributeAsString := "[\n"
	for i, item := range attributes {
		jsonByte, err := json.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("error marshalling attribute item: %w", err)
		}
		jsonString := string(jsonByte)
		if i != len(attributes)-1 {
			jsonString += ","
		}
		attributeAsString += jsonString + "\n"
	}
	attributeAsString += "]"

	return []byte(processEncodedString(attributeAsString)), nil
}
