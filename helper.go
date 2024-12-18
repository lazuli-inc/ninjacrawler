package ninjacrawler

import (
	"bytes"
	"cloud.google.com/go/compute/metadata"
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
	"github.com/playwright-community/playwright-go"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"plugin"
	"regexp"
	"runtime/debug"
	"strings"
	"sync/atomic"
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

func (app *Crawler) GetDocument(data interface{}) (*goquery.Document, error) {
	var document *goquery.Document
	var err error

	switch v := data.(type) {
	case playwright.Page:
		document, err = app.GetPageDom(v)
		if err != nil {
			return nil, err
		}
	case *rod.Page:
		document, err = app.GetRodPageDom(v)
		if err != nil {
			return nil, err
		}
	default:
	}
	return document, nil
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
func (app *Crawler) GetRodPageDom(page *rod.Page) (*goquery.Document, error) {
	html, err := page.HTML()
	if err != nil {
		return nil, err
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	return document, nil
}
func (app *Crawler) writePageContentToFile(html, url, msg string, dir ...string) error {
	if html == "" {
		html = "No Page Content Found"
	}
	html = strings.TrimSpace(msg) + "\n" + html
	html = fmt.Sprintf("<!-- Time: %v \n Page Url: %s -->\n%s", time.Now(), url, html)
	filename := generateFilename(url)
	directory := filepath.Join("storage", "logs", app.Name, "html")
	if dir != nil && len(dir) > 0 {
		directory = filepath.Join("storage", "logs", app.Name, "html", dir[0])
	}
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
func (app *Crawler) writeRawHtmlFile(html, url string) error {
	if html == "" {
		html = "No Page Content Found"
	}
	html = fmt.Sprintf("<!-- Time: %v \n Page Url: %s -->\n%s", time.Now(), url, html)
	filename := generateRawFilename(url)
	currentDate := time.Now().Format("2006-01-02")
	directory := filepath.Join("storage", "raw_html", app.Name, currentDate)
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

// generateFilename generates a filename based on URL and current date
func generateRawFilename(rawURL string) string {
	// Replace characters not allowed in file names
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalidChars {
		rawURL = strings.ReplaceAll(rawURL, char, "_")
	}

	// Maximum filename length is 255 characters, minus 5 characters for ".html"
	maxFilenameLength := 255 - 5

	// Trim the URL to fit within the maximum length
	trimmedURL := rawURL
	if len(trimmedURL) > maxFilenameLength {
		trimmedURL = trimmedURL[:maxFilenameLength]
	}

	// Remove trailing underscore if it exists
	trimmedURL = strings.TrimSuffix(trimmedURL, "_")

	return fmt.Sprintf("%s.html", trimmedURL)
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
		app.Logger.Error("failed to get html from page: %s", err.Error())
	}
	return html
}

func (app *Crawler) getHtmlFromRodPage(page *rod.Page) string {
	html, err := page.HTML()
	if err != nil {
		app.Logger.Error("failed to get html from page: %s", err.Error())
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

func (app *Crawler) HandleThrottling(attempt, StatusCode int) {
	if attempt > 0 {
		sleepDuration := time.Duration(app.engine.RetrySleepDuration) * time.Minute * time.Duration(attempt)
		app.Logger.Debug("Sleeping for %s StatusCode: %d", sleepDuration, StatusCode)
		time.Sleep(sleepDuration)
	}
}

type comparableType interface {
	int | string
}

// Generic contains function
func inArray[T comparableType](arr []T, val T) bool {
	if arr == nil || len(arr) == 0 {
		return false
	}
	for _, v := range arr {
		if v == val {
			return true
		}
	}
	return false
}

func (app *Crawler) GetHtml(data interface{}) (string, error) {
	var htmlContent string
	var err error

	switch v := data.(type) {
	case playwright.Page:
		htmlContent = app.getHtmlFromPage(v)
	case *rod.Page:
		htmlContent = app.getHtmlFromRodPage(v)
	case []byte:
		htmlContent = string(v)
	case *goquery.Document:
		htmlContent, err = v.Html() // Fetch HTML from goquery document
		if err != nil {
			return htmlContent, fmt.Errorf("Failed to get content from goquery document in SaveHtml %v", err.Error())
		}
	default:
	}
	return htmlContent, nil
}
func (app *Crawler) SaveHtml(data interface{}, urlString string) error {
	htmlContent, err := app.GetHtml(data)
	err = app.writeRawHtmlFile(htmlContent, urlString)
	if err != nil {
		return fmt.Errorf("Failed to write html content to file in SaveHtml %v", err.Error())
	}
	return nil
}

func (app *Crawler) SendHtmlToBigquery(data interface{}, urlString string) error {
	htmlContent, err := app.GetHtml(data)
	if err != nil {
		return fmt.Errorf("failed to get html %s", err.Error())
	}

	if metadata.OnGCE() {
		bigqueryErr := app.sendHtmlToBigquery(htmlContent, urlString)
		if bigqueryErr != nil {
			bigErr := app.markAsBigQueryFailed(urlString, bigqueryErr.Error())
			if bigErr != nil {
				return bigErr
			}
			return bigqueryErr

		}
	}
	return nil
}
func (app *Crawler) HandlePanic(r any) {
	if r != nil {
		app.Logger.Summary("Program crashed!: %+v", r)
		app.Logger.Debug("Panic caught: %+v", r)
		app.Logger.Debug("Stack trace: \n%s", string(debug.Stack()))
		os.Exit(1)
	}
}

func (app *Crawler) StopCrawler() error {
	if !metadata.OnGCE() {
		return nil
	}

	instanceName, err := metadata.InstanceName()
	if err != nil {
		log.Fatalf("Failed to get instanceName: %v", err)
	}

	managerUrl := fmt.Sprintf("%s/api/stop-crawler/%s", app.Config.GetString("SERVER_IP"), instanceName)

	client := &http.Client{}
	req, err := http.NewRequest("GET", managerUrl, nil)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection Failed: %w", err)
	}
	defer response.Body.Close()

	// Check for non-200 status codes
	if response.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(response.Body)
		return fmt.Errorf("api error status %d, body: %s", response.StatusCode, string(bodyBytes))
	}
	return nil
}
func (app *Crawler) AddCrawlingLog(payload string) error {
	if !metadata.OnGCE() {
		return nil
	}
	instanceName := "local-vm-" + app.Name + "-" + time.Now().Format("2006-01-02")
	var err error
	if metadata.OnGCE() {
		instanceName, err = metadata.InstanceName()
		if err != nil {
			log.Fatalf("Failed to get instanceName: %v", err)
		}
	}

	managerUrl := fmt.Sprintf("%s/api/add-crawler-logs/%s", app.Config.GetString("SERVER_IP"), instanceName)

	client := &http.Client{}
	req, err := http.NewRequest("POST", managerUrl, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(response.Body)
		return fmt.Errorf("API error status %d, body: %s", response.StatusCode, string(bodyBytes))
	}

	app.Logger.Debug("Monitoring report successfully sent to Crawl Manager API.")
	return nil
}
func (app *Crawler) AddCrawSummary(payload string) error {
	if !metadata.OnGCE() {
		return nil
	}
	instanceName := "local-vm-" + app.Name + "-" + time.Now().Format("2006-01-02")
	var err error
	if metadata.OnGCE() {
		instanceName, err = metadata.InstanceName()
		if err != nil {
			log.Fatalf("Failed to get instanceName: %v", err)
		}
	}

	managerUrl := fmt.Sprintf("%s/api/add-crawler-summary/%s", app.Config.GetString("SERVER_IP"), instanceName)

	client := &http.Client{}
	req, err := http.NewRequest("POST", managerUrl, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(response.Body)
		return fmt.Errorf("API error status %d, body: %s", response.StatusCode, string(bodyBytes))
	}

	app.Logger.Debug("Summary report successfully sent to Crawl Manager API.")
	return nil
}

func (app *Crawler) FetchProxy() ([]Proxy, error) {
	var proxies []Proxy

	// Prepare the API request URL
	managerURL := fmt.Sprintf("%s/api/proxy/%s", app.Config.GetString("SERVER_IP"), app.Name)

	app.Logger.Debug("Fetching proxies from %s", managerURL)
	// Create an HTTP client and request
	client := &http.Client{}
	req, err := http.NewRequest("GET", managerURL, nil)
	if err != nil {
		return proxies, fmt.Errorf("request failed: %w", err)
	}

	// Send the request
	response, err := client.Do(req)
	if err != nil {
		return proxies, fmt.Errorf("connection failed: %w", err)
	}
	defer response.Body.Close()

	// Check for non-200 status codes
	if response.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(response.Body)
		return proxies, fmt.Errorf("API error status %d, body: %s", response.StatusCode, string(bodyBytes))
	}

	// Define a struct to parse the API response
	var apiResponse struct {
		Data    []Proxy `json:"data"`
		Success bool    `json:"success"`
	}

	// Parse the JSON response
	if err := json.NewDecoder(response.Body).Decode(&apiResponse); err != nil {
		return proxies, fmt.Errorf("error decoding response: %w", err)
	}

	// Return the proxies if the response was successful
	if apiResponse.Success {
		return apiResponse.Data, nil
	}

	return proxies, fmt.Errorf("API response indicated failure")
}

func (app *Crawler) stopProxy(proxy Proxy, errStr string) error {
	app.Logger.Debug("Stopping proxy: %s", errStr)

	// Prepare the API request URL
	managerURL := fmt.Sprintf("%s/api/proxy/stop", app.Config.GetString("SERVER_IP"))

	// Prepare the payload with proxy and error message
	payload := map[string]interface{}{
		"proxy": proxy,
		"error": errStr,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create an HTTP client and request
	client := &http.Client{}
	req, err := http.NewRequest("POST", managerURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("request creation failed: %w", err)
	}

	// Set headers for JSON content
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer response.Body.Close()

	// Check for non-200 status codes
	if response.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(response.Body)
		return fmt.Errorf("API error status %d, payload %s, body: %s", response.StatusCode, string(jsonData), string(bodyBytes))
	}

	return nil
}

func (app *Crawler) handleHttpError(statusCode int, statusText string, url string, data interface{}) error {

	msg := fmt.Sprintf("failed to fetch page: StatusCode: %v, Status: %v", statusCode, statusText)

	// Handle 404 error
	if statusCode == http.StatusNotFound {
		_ = app.MarkAsMaxErrorAttempt(url, app.CurrentCollection, "Url Not Found")
		return fmt.Errorf("url Not Found: StatusCode %v", statusCode)
	}

	// Handle retryable error codes
	if inArray(app.engine.ErrorCodes, statusCode) {
		msg = fmt.Sprintf("isRetryable: StatusCode: %v, Status: %v", statusCode, statusText)
		app.Logger.Error(msg)
		app.Logger.Debug("Got Blocked at URL: %s Error: %v\n", app.CurrentUrl, msg)
	} else {
		app.Logger.Debug("Http Error URL: %s Error: %v\n", url, msg)
	}
	htmlStr, err := app.GetHtml(data)
	if err != nil {
		return err
	}
	app.Logger.Html(htmlStr, url, msg)
	return fmt.Errorf(msg)
}
func (app *Crawler) handleProxyError(proxy Proxy, err error) (*goquery.Document, error) {
	if strings.Contains(err.Error(), "net::ERR_HTTP_RESPONSE_CODE_FAILURE") ||
		strings.Contains(err.Error(), "net::ERR_INVALID_AUTH_CREDENTIALS") ||
		strings.Contains(err.Error(), "Proxy Authentication Required") ||
		strings.Contains(err.Error(), "Could not connect to proxy server") ||
		strings.Contains(err.Error(), "NS_ERROR_PROXY_CONNECTION_REFUSED") {
		stopErr := app.stopProxy(proxy, err.Error())
		if stopErr != nil {
			return nil, stopErr
		}
		app.syncProxies()
		return nil, fmt.Errorf("isRetryable: Unexpected Error: %s", err.Error())
	}
	return nil, fmt.Errorf("failed to navigate %s", err.Error())
}
func ensureScheme(rawUrl string) (string, error) {
	// Check if the URL starts with a scheme (http://, https://, etc.)
	if !strings.Contains(rawUrl, "://") {
		// If there is no scheme, default to http
		rawUrl = "http://" + rawUrl
	}

	// Now parse the URL
	parsedUrl, err := url.Parse(rawUrl)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	return parsedUrl.String(), nil
}

func (app *Crawler) getCurrentProxy() Proxy {
	locker.Lock()
	defer locker.Unlock()
	proxy := Proxy{}
	if len(app.engine.ProxyServers) == 0 && app.engine.ProxyStrategy == ProxyStrategyRotation {
		app.Logger.Fatal("No Active proxy servers found")
	}

	if len(app.engine.ProxyServers) > 0 {
		index := atomic.LoadInt32(&app.CurrentProxyIndex) % int32(len(app.engine.ProxyServers))
		proxy = app.engine.ProxyServers[index]
	}
	return proxy
}
func (app *Crawler) cleanUpTempFiles() error {
	rodTempDir := "/tmp/rod/user-data"

	// Check if the directory exists before attempting to clean it
	if _, err := os.Stat(rodTempDir); os.IsNotExist(err) {
		// Directory doesn't exist, no need to clean
		return nil
	}

	// Walk through the /tmp/rod directory and remove its contents recursively
	err := filepath.Walk(rodTempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// If the file no longer exists, ignore the error
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		// Remove files and directories
		if !info.IsDir() {
			// Attempt to remove the file
			if err := os.Remove(path); err != nil {
				if os.IsNotExist(err) {
					return nil // Ignore the missing file and continue
				}
				return fmt.Errorf("failed to remove file %s: %w", path, err)
			}
		} else {
			// Continue the walk and delete directories last
			return nil
		}
		return nil
	})

	// Once all files are removed, attempt to remove the directories
	if err == nil {
		err = os.RemoveAll(rodTempDir)
		if err != nil {
			return fmt.Errorf("failed to remove directory %s: %w", rodTempDir, err)
		}
	}

	if err != nil {
		return err
	}
	return nil
}

func (app *Crawler) GetUserAgent() string {
	return app.userAgent
}

// Helper function to fill input fields based on cookie consent action
func fillConsentFields(page interface{}, action *CookieAction) error {
	if action == nil || len(action.Fields) == 0 {
		return nil
	}

	for _, field := range action.Fields {
		inputSelector := fmt.Sprintf("input[name='%s']", field.Key)

		switch p := page.(type) {
		case playwright.Page:
			input, err := p.QuerySelector(inputSelector)
			if err != nil {
				return fmt.Errorf("failed to find input field with name '%s': %w", field.Key, err)
			}
			if input != nil {
				err = input.Fill(field.Val)
				if err != nil {
					return fmt.Errorf("failed to fill input field with name '%s': %w", field.Key, err)
				}
			}
		case *rod.Page:
			el := p.MustElement(inputSelector)
			el.MustInput(field.Val)
		}
	}
	return nil
}

// Helper function to click the cookie consent button
func clickConsentButton(page interface{}, action *CookieAction) error {
	if action == nil || (action.Selector == "" && action.ButtonText == "") {
		return nil
	}

	// Determine the selector to use: `action.Selector` takes precedence over `action.ButtonText`.
	var buttonSelector string
	if action.Selector != "" {
		buttonSelector = action.Selector
	} else {
		buttonSelector = fmt.Sprintf("button:has-text('%s')", action.ButtonText)
	}

	switch p := page.(type) {
	case playwright.Page:
		// Wait for the button to appear using the chosen selector
		if _, err := p.WaitForSelector(buttonSelector); err != nil {
			return fmt.Errorf("failed to find button selector %s: %w", buttonSelector, err)
		}

		// Query for the button and attempt to click it
		button, err := p.QuerySelector(buttonSelector)
		if err != nil || button == nil {
			return fmt.Errorf("failed to find or click cookie consent button: %w", err)
		}
		if err := button.Click(); err != nil {
			return fmt.Errorf("failed to click cookie consent button: %w", err)
		}

		// Wait for the next element after clicking
		if action.MustHaveSelectorAfterAction != "" {
			if _, err := p.WaitForSelector(action.MustHaveSelectorAfterAction); err != nil {
				return fmt.Errorf("failed waiting for post-action selector: %w", err)
			}
		}

	case *rod.Page:
		// Locate the button using the selector or text and click
		var button *rod.Element
		if action.Selector != "" {
			button = p.MustElement(action.Selector)
		} else {
			button = p.MustElementR("button", action.ButtonText)
		}
		if button == nil {
			return fmt.Errorf("failed to find button with selector/text: %s", buttonSelector)
		}
		button.MustClick()

		// Wait for the page to load after clicking
		p.MustWaitLoad()

	default:
		return fmt.Errorf("unsupported page type: %T", p)
	}

	return nil
}

// Common function to handle cookie consent logic
func handleCookieConsent(page interface{}, action *CookieAction) error {
	if err := fillConsentFields(page, action); err != nil {
		return err
	}
	return clickConsentButton(page, action)
}

// Helper functions for setting user-agent and Sec-CH-UA headers
func setChromiumHeaders(opts *playwright.BrowserNewContextOptions, browser playwright.Browser) {
	version := extractMajorVersion(browser.Version())
	opts.ExtraHttpHeaders["User-Agent"] = fmt.Sprintf("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", version)
	opts.ExtraHttpHeaders["Sec-Ch-Ua"] = fmt.Sprintf("\"Not/A)Brand\";v=\"99\", \"Chromium\";v=\"%s\", \"Google Chrome\";v=\"%s\"", version, version)
}

func setFirefoxHeaders(opts *playwright.BrowserNewContextOptions, browser playwright.Browser) {
	version := extractMajorVersion(browser.Version())
	opts.ExtraHttpHeaders["User-Agent"] = fmt.Sprintf("Mozilla/5.0 (X11 Ubuntu; Linux x86_64; rv:%s) Gecko/20100101 Firefox/%s", version, version)
}

func setWebKitHeaders(opts *playwright.BrowserNewContextOptions, browser playwright.Browser) {
	version := extractMajorVersion(browser.Version())
	opts.ExtraHttpHeaders["User-Agent"] = fmt.Sprintf("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/%s Safari/605.1.15", version)
}

func extractMajorVersion(version string) string {
	return strings.Split(version, ".")[0]
}
