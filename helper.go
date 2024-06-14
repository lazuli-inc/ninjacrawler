package ninjacrawler

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/playwright-community/playwright-go"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func isLocalEnv() bool {
	return app.Config.GetString("APP_ENV") == "local"
}

// processDocument processes the document based on the urlSelector type
func processDocument(doc *goquery.Document, selector UrlSelector, collection UrlCollection) []UrlCollection {
	if selector.SingleResult {
		// Process a single result
		return processSingleResult(doc, selector, collection)
	} else {
		// Process multiple results
		var items []UrlCollection

		doc.Find(selector.Selector).Each(func(i int, selection *goquery.Selection) {
			item := processSelection(selection, selector, collection)
			items = append(items, item...)
		})

		return items
	}
}

// processSingleResult processes a single result based on the selector
func processSingleResult(doc *goquery.Document, selector UrlSelector, urlCollection UrlCollection) []UrlCollection {
	selection := doc.Find(selector.Selector).First()
	return processSelection(selection, selector, urlCollection)
}

// processSelection processes each selection and extracts attribute values
func processSelection(selection *goquery.Selection, selector UrlSelector, collection UrlCollection) []UrlCollection {
	var items []UrlCollection

	selection.Find(selector.FindSelector).Each(func(j int, s *goquery.Selection) {
		attrValue, ok := s.Attr(selector.Attr)
		if !ok {
			app.Logger.Error("Attribute not found. %v", selector.Attr)
		} else {
			fullUrl := GetFullUrl(attrValue)
			if selector.Handler != nil {
				url, meta := selector.Handler(collection, fullUrl, s)
				if url != "" {
					items = append(items, UrlCollection{
						Url:      url,
						MetaData: meta,
						Parent:   collection.Parent,
					})
				}
			} else {
				items = append(items, UrlCollection{
					Url:      fullUrl,
					MetaData: nil,
					Parent:   collection.Parent,
				})
			}
		}
	})

	return items
}

func getItemsFromAttrOrText(selection *goquery.Selection, selector *CategorySelector) []string {
	var items []string
	selection.Each(func(i int, s *goquery.Selection) {
		var value string
		var ok bool // Declare ok here to avoid shadowing
		s.Find("span.gt").Remove()
		if selector.Attr != "" {
			if value, ok = s.Attr(selector.Attr); ok {
				items = append(items, value)
			}
		} else {
			value = strings.TrimSpace(s.Text())
			items = append(items, value)
		}
	})
	return items
}

func GetPageDom(page playwright.Page) (*goquery.Document, error) {
	html, err := page.Content()
	if err != nil {
		return nil, err
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	return document, nil
}
func writePageContentToFile(page playwright.Page, msg string) error {
	content, err := page.Content()
	if err != nil {
		content = "No Page Content Found \n" + strings.TrimSpace(msg)
	}
	content = fmt.Sprintf("<!-- Time: %v \n Page Url: %s -->\n%s", time.Now(), page.URL(), content)
	filename := generateFilename(page.URL())
	directory := filepath.Join("storage", "logs", app.Name, "html")
	err = os.MkdirAll(directory, 0755)
	if err != nil {
		return err
	}
	filePath := filepath.Join(directory, filename)
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	if err != nil {
		return err
	}

	return nil
}

// generateFilename generates a filename based on URL and current date
func generateFilename(rawURL string) string {

	// Replace remaining characters not allowed in file names
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalidChars {
		rawURL = strings.ReplaceAll(rawURL, char, "_")
	}

	// Combine the encoded path with current date and a suitable extension
	currentDate := time.Now().Format("2006-01-02")
	return currentDate + "_" + rawURL + ".html"
}
func generateCsvFileName(siteName string) string {
	productDetailsFileName := fmt.Sprintf("storage/data/%s/%s.csv", siteName, time.Now().Format("2006_01_02"))

	return productDetailsFileName
}

func GetFullUrl(url string) string {
	fullUrl := ""
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		// If href is already a full URL, don't concatenate with baseUrl
		fullUrl = url
	} else {
		fullUrl = app.BaseUrl + url
	}
	return fullUrl
}

// shouldBlockResource checks if a resource should be blocked based on its type and URL.
func shouldBlockResource(resourceType string, url string) bool {
	if resourceType == "image" || resourceType == "font" {
		return true
	}

	for _, blockedURL := range app.engine.BlockedURLs {
		if strings.Contains(url, blockedURL) {
			return true
		}
	}

	return false
}
func getBaseUrl(urlString string) string {
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		app.Logger.Error("failed to parse Url:", "Error", err)
		return ""
	}

	baseURL := parsedURL.Scheme + "://" + parsedURL.Host
	return baseURL
}
