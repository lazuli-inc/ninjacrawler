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

func isLocalEnv(env string) bool {
	return env == "local"
}

// processDocument processes the document based on the urlSelector type
func (app *Crawler) processDocument(doc *goquery.Document, selector UrlSelector, collection UrlCollection) []UrlCollection {
	if selector.SingleResult {
		// Process a single result
		return app.processSingleResult(doc, selector, collection)
	} else {
		// Process multiple results
		var items []UrlCollection

		doc.Find(selector.Selector).Each(func(i int, selection *goquery.Selection) {
			item := app.processSelection(selection, selector, collection)
			items = append(items, item...)
		})

		return items
	}
}

// processSingleResult processes a single result based on the selector
func (app *Crawler) processSingleResult(doc *goquery.Document, selector UrlSelector, urlCollection UrlCollection) []UrlCollection {
	selection := doc.Find(selector.Selector).First()
	return app.processSelection(selection, selector, urlCollection)
}

// processSelection processes each selection and extracts attribute values
func (app *Crawler) processSelection(selection *goquery.Selection, selector UrlSelector, collection UrlCollection) []UrlCollection {
	var items []UrlCollection

	selection.Find(selector.FindSelector).Each(func(j int, s *goquery.Selection) {
		attrValue, ok := s.Attr(selector.Attr)
		if !ok {
			app.Logger.Error("Attribute not found. %v", selector.Attr)
		} else {
			fullUrl := app.GetFullUrl(attrValue)
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

func (app *Crawler) GetPageDom(page playwright.Page) (*goquery.Document, error) {
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
func (app *Crawler) writePageContentToFile(page playwright.Page, msg string) error {
	content, err := page.Content()
	if err != nil {
		content = "No Page Content Found"
	}
	content = strings.TrimSpace(msg) + "\n" + content
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

func (app *Crawler) GetFullUrl(url string) string {
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
func (app *Crawler) shouldBlockResource(resourceType string, url string) bool {
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
func (app *Crawler) getBaseUrl(urlString string) string {
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		app.Logger.Error("failed to parse Url:", "Error", err)
		return ""
	}

	baseURL := parsedURL.Scheme + "://" + parsedURL.Host
	return baseURL
}
