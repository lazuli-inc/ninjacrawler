package ninjacrawler

import (
	"cloud.google.com/go/compute/metadata"
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/playwright-community/playwright-go"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"plugin"
	"regexp"
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
func (app *Crawler) writePageContentToFile(html, url, msg string) error {
	if html == "" {
		html = "No Page Content Found"
	}
	html = strings.TrimSpace(msg) + "\n" + html
	html = fmt.Sprintf("<!-- Time: %v \n Page Url: %s -->\n%s", time.Now(), url, html)
	filename := generateFilename(url)
	directory := filepath.Join("storage", "logs", app.Name, "html")
	err := os.MkdirAll(directory, 0755)
	if err != nil {
		return err
	}
	filePath := filepath.Join(directory, filename)
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(html)
	if err != nil {
		return err
	}

	return nil
}

// generateFilename generates a filename based on URL and current date
func generateFilename(rawURL string) string {
	// Hash the URL to create a unique, short identifier
	hasher := sha1.New()
	hasher.Write([]byte(rawURL))
	hash := fmt.Sprintf("%x", hasher.Sum(nil)) // Convert hash to hex string

	// Replace characters not allowed in file names
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalidChars {
		rawURL = strings.ReplaceAll(rawURL, char, "_")
	}

	// Trim the URL to a maximum length (e.g., 50 characters)
	trimmedURL := rawURL
	if len(trimmedURL) > 50 {
		trimmedURL = trimmedURL[:50]
	}

	// Combine the trimmed URL with the hash and current date
	currentDate := time.Now().Format("2006-01-02")
	return fmt.Sprintf("%s_%s_%s.html", currentDate, trimmedURL, hash)
}
func generateCsvFileName(siteName string) string {
	productDetailsFileName := fmt.Sprintf("storage/data/%s/%s.csv", siteName, time.Now().Format("2006_01_02"))

	return productDetailsFileName
}

func (app *Crawler) GetFullUrl(url string) string {

	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		// If url is already a full URL, return it as is
		return url
	} else if strings.HasPrefix(url, "//") {
		// If url starts with "//", use the protocol from BaseUrl
		if strings.HasPrefix(app.BaseUrl, "https://") {
			return "https:" + url
		}
		return "http:" + url
	}
	// If the URL doesn't start with "/", add the "/products" prefix
	if !strings.HasPrefix(url, "/") {
		url = "/" + url
	}

	// Otherwise, concatenate with BaseUrl
	return app.BaseUrl + url
}
func (app *Crawler) HtmlToText(selection *goquery.Selection) string {
	trimIndex := []int{}

	textSlice := selection.Contents().Map(func(i int, s *goquery.Selection) string {
		tagName := strings.ToLower(goquery.NodeName(s))
		if s.ChildrenFiltered("*").Length() == 0 {
			if tagName == "br" {
				return "\n"
			}
			if tagName == "img" {
				src, ok := s.Attr("src")
				if ok {
					if !strings.HasPrefix(src, "http") {
						src = app.GetFullUrl(src)
					}
				}
				return src + "\n"
			}
			if tagName == "style" {
				return "\n"
			}

			text := s.Text()
			text = strings.ReplaceAll(text, "\u00A0", " ")
			text = strings.Trim(text, " \t\n")
			if len(text) > 0 {
				text += "\n"
			}
			if tagName == "sup" || tagName == "sub" {
				if i-1 > 0 {
					trimIndex = append(trimIndex, i-1)
				}
				text = strings.Trim(text, "\n")
			}

			return text
		} else if tagName == "table" {
			return app.GetTableDataAsString(s) + "\n"
		} else {
			return app.HtmlToText(s) + "\n"
		}
	})

	for _, i := range trimIndex {
		textSlice[i] = strings.Trim(textSlice[i], " \n")
	}

	return strings.Trim(strings.Join(textSlice, ""), " \n")
}
func (app *Crawler) GetTableDataAsString(selection *goquery.Selection) string {
	data := ""

	selection.Find("tr").Each(func(i int, s *goquery.Selection) {
		items := []string{}
		s.Children().Each(func(j int, h *goquery.Selection) {
			items = append(items, app.HtmlToText(h))
		})

		if len(items) > 0 {
			data += strings.Join(items, "/") + "\n"
			return
		}
	})
	data = strings.Trim(data, " \n")

	return data
}
func (app *Crawler) GetQueryEscapeFullUrl(urlStr string) string {
	var fullUrl string
	if strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://") {
		// If url is already a full URL, return it as is
		fullUrl = urlStr
	} else if strings.HasPrefix(urlStr, "//") {
		// If url starts with "//", use the protocol from BaseUrl
		if strings.HasPrefix(app.BaseUrl, "https://") {
			fullUrl = "https:" + urlStr
		} else {
			fullUrl = "http:" + urlStr
		}
	} else {
		// Otherwise, concatenate with BaseUrl
		fullUrl = app.BaseUrl + urlStr
	}

	parsedUrl, err := url.Parse(fullUrl)
	if err != nil {
		return ""
	}

	// Encode only the query part, not the entire URL
	parsedUrl.RawQuery = parsedUrl.Query().Encode()
	parsedUrl.RawQuery = url.QueryEscape(parsedUrl.RawQuery)
	encodedURL := parsedUrl.String()
	return encodedURL
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

func (app *Crawler) getHtmlFromPage(page playwright.Page) string {
	html, err := page.Content()
	if err != nil {
		app.Logger.Error("failed to get html from page", "Error", err)
	}
	return html
}

func (app *Crawler) LoadSites(filename string) ([]CrawlerConfig, error) {
	var sites []CrawlerConfig

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&sites)
	if err != nil {
		return nil, fmt.Errorf("error decoding JSON: %w", err)
	}

	return sites, nil
}

func (app *Crawler) buildPlugin(path, filename, functionName string) {
	src := filepath.Join(path, filename)
	dest := filepath.Join(path, functionName+".so")

	if err := app.generatePlugin(src, dest); err != nil {
		fmt.Printf("Failed to build plugin %s: %v\n", functionName, err)
		os.Exit(1)
	}

	fmt.Println("All plugins built successfully.")
}
func (app *Crawler) generatePlugin(src, dest string) error {
	cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", dest, src)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	return nil
}

func loadHandler(pluginPath, funcName string) (plugin.Symbol, error) {
	// Load the package
	pkg, err := plugin.Open(pluginPath)
	if err != nil {
		return nil, fmt.Errorf("error opening package: %w", err)
	}

	// Look for the function by name in the package
	fnSymbol, err := pkg.Lookup(funcName)
	if err != nil {
		return nil, fmt.Errorf("function %s not found in package %s: %w", funcName, pluginPath, err)
	}

	return fnSymbol, nil
}
func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

func ExecuteCommand(command string, args []string) string {
	cmd := exec.Command(command, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	return string(output)
}

func StopInstanceIfRunningFromGCP() {
	if metadata.OnGCE() {
		fmt.Println("Running on GCP VM")

		projectID, err := metadata.ProjectID()
		if err != nil {
			log.Fatalf("Failed to get project ID: %v", err)
		}

		zone, err := metadata.Zone()
		if err != nil {
			log.Fatalf("Failed to get zone: %v", err)
		}

		instanceID, err := metadata.InstanceID()
		if err != nil {
			log.Fatalf("Failed to get instance ID: %v", err)
		}

		ctx := context.Background()
		computeService, err := compute.NewService(ctx, option.WithScopes(compute.CloudPlatformScope))
		if err != nil {
			log.Fatalf("Failed to create compute service: %v", err)
		}

		// Stop the VM
		_, err = computeService.Instances.Stop(projectID, zone, instanceID).Context(ctx).Do()
		if err != nil {
			log.Fatalf("Failed to stop instance: %v", err)
		}

		fmt.Println("VM has been stopped")
	}
}
func (app *Crawler) ToNumericsString(str string) string {
	return regexp.MustCompile(`[^0-9]`).ReplaceAllString(str, "")
}

// checkContentType sends a HEAD request to the URL and checks the Content-Type header
func (app *Crawler) CheckContentType(url string) (string, error) {
	resp, err := http.Head(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	ContentType := resp.Header.Get("Content-Type")
	return ContentType, nil
}

// IsHTMLPage checks if the Content-Type indicates an HTML page
//
//	func (app *Crawler) IsHTMLPage(urlStr string) bool {
//		ContentType, err := app.CheckContentType(urlStr)
//		if err != nil {
//			return false
//		}
//		return strings.Contains(ContentType, "text/html")
//	}
//
// IsHTMLPage checks if the URL has an HTML-related file extension
func (app *Crawler) IsValidPage(urlStr string) bool {
	var htmlExtensions = []string{".html", ".htm", ".php", ".asp", ".aspx", ".jsp"}
	ext := app.GetUrlFileExtension(urlStr)
	// Check if the URL has a valid HTML extension
	for _, pExt := range htmlExtensions {
		if ext == "" || ext == "." || ext == pExt {
			return true
		}
	}
	return false
}
func (app *Crawler) GetUrlFileExtension(urlString string) string {
	u, err := url.Parse(urlString)
	if err != nil {
		fmt.Println("Error parsing URL:", err)
		return ""
	}
	fileName := path.Base(u.Path)
	extension := filepath.Ext(fileName)

	return extension
}

func (app *Crawler) handleThrottling(attempt, StatusCode int) {
	if attempt > 0 {
		sleepDuration := time.Duration(app.engine.RetrySleepDuration) * time.Minute * time.Duration(attempt)
		app.Logger.Debug("Sleeping for %s StatusCode: %d", sleepDuration, StatusCode)
		time.Sleep(sleepDuration)
	}
}
