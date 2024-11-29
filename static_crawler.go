package ninjacrawler

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/charset"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (app *Crawler) GetHttpClient() *http.Client {
	client := &http.Client{
		Timeout: app.engine.Timeout,
	}
	return client
}

func (app *Crawler) NavigateToStaticURL(client *http.Client, urlString string, proxyServer Proxy) (*goquery.Document, error) {
	body, ContentType, err := app.getResponseBody(client, urlString, proxyServer, 0)
	if err != nil {
		return nil, err
	}

	// Create a reader that can decode the response body with the correct encoding
	reader, err := charset.NewReader(strings.NewReader(string(body)), ContentType)
	if err != nil {
		return nil, fmt.Errorf("failed to create reader with correct encoding: %w", err)
	}

	document, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, err
	}

	if app.engine.SendHtmlToBigquery != nil && *app.engine.SendHtmlToBigquery {
		sendErr := app.SendHtmlToBigquery(document, urlString)
		if sendErr != nil {
			app.Logger.Error("SendHtmlToBigquery Error: %s", sendErr.Error())
		}
	}

	if *app.engine.StoreHtml {
		if StoreHtmlErr := app.SaveHtml(document, urlString); StoreHtmlErr != nil {
			app.Logger.Error(StoreHtmlErr.Error())
		}
	}
	return document, nil
}

func (app *Crawler) NavigateToApiURL(client *http.Client, urlString string, proxyServer Proxy) (map[string]interface{}, error) {
	body, _, err := app.getResponseBody(client, urlString, proxyServer, 0)
	if err != nil {
		return nil, err
	}

	// Decode JSON response
	var jsonResponse map[string]interface{}
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON response: %w", err)
	}

	return jsonResponse, nil
}

func (app *Crawler) getResponseBody(client *http.Client, urlString string, proxyServer Proxy, attempt int) ([]byte, string, error) {
	app.mu.Lock()         // Lock before accessing/modifying shared state
	defer app.mu.Unlock() // Unlock when the function returns
	app.CurrentUrl = urlString
	ContentType := ""
	//proxyIp := ""
	originalUrl := urlString

	httpTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   90 * time.Second,
			KeepAlive: 90 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   90 * time.Second,
		ResponseHeaderTimeout: 90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
		},
	}

	if app.engine.Provider == "zenrows" {

		urlString = app.GetQueryEscapeFullUrl(urlString)
		zenrowsApiKey := app.Config.EnvString("ZENROWS_API_KEY")
		queryString := app.BuildQueryString()
		urlString = fmt.Sprintf("https://api.zenrows.com/v1/?apikey=%s&url=%s&%s", zenrowsApiKey, urlString, queryString)

	} else {
		if len(app.engine.ProxyServers) > 0 && proxyServer.Server != "" {
			// Ensure the proxy URL has a scheme, defaulting to http:// if absent
			proxyServerUrl, err := ensureScheme(proxyServer.Server)
			if err != nil {
				log.Fatalf("Failed to ensure scheme for proxy URL: %v", err)
			}

			// Parse the proxy URL
			proxyURL, err := url.Parse(proxyServerUrl)
			if err != nil {
				log.Fatalf("Failed to parse proxy URL: %v", err)
			}
			// Set proxy credentials if available
			if proxyServer.Username != "" && proxyServer.Password != "" {
				proxyURL.User = url.UserPassword(proxyServer.Username, proxyServer.Password)
			}

			// Set the proxy in the transport
			httpTransport.Proxy = http.ProxyURL(proxyURL)

			if proxyURL.Scheme == "http" || proxyURL.Scheme == "https" {
				httpTransport.TLSClientConfig = &tls.Config{
					InsecureSkipVerify: true,
				}
			}

			client.Transport = httpTransport
		}
	}
	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		return nil, ContentType, fmt.Errorf("Failed to create request: %v", err)
	}
	userAgent := "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"
	// Overwrite the default User-Agent header
	if app.userAgent != "" {
		userAgent = app.GetUserAgent()
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", app.BaseUrl)

	resp, err := client.Do(req)
	if err != nil {
		errMsg := fmt.Sprintf("failed to navigate %s", err.Error())
		if strings.Contains(err.Error(), "Client.Timeout") {
			_ = app.updateStatusCode(originalUrl, 408)
			return nil, ContentType, fmt.Errorf(errMsg)
		}
		if strings.Contains(err.Error(), "Too Many Requests") {
			if inArray(app.engine.ErrorCodes, 429) {
				errMsg = fmt.Sprintf("isRetryable : Too Many Requests: %v", err)
			}
			_ = app.updateStatusCode(originalUrl, 429)
			return nil, ContentType, fmt.Errorf(errMsg)
		}
		_, e := app.handleProxyError(proxyServer, err)
		return nil, ContentType, e
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ContentType, fmt.Errorf("failed to read response body: %w", err)
	}
	ContentType = resp.Header.Get("Content-Type")
	_ = app.updateStatusCode(originalUrl, resp.StatusCode)
	// Check if a redirect occurred
	if req.URL.String() != resp.Request.URL.String() && *app.engine.TrackRedirection {
		resUrl, _ := url.Parse(resp.Request.URL.String())
		if resUrl.Host != req.Host {
			finalURL := resp.Request.URL.String()
			app.CurrentUrl = finalURL
			_ = app.updateRedirection(originalUrl, finalURL)
			app.Logger.Warn(fmt.Sprintf("Redirection detected: %s -> %s", originalUrl, finalURL))
		}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ContentType, app.handleHttpError(resp.StatusCode, resp.Status, originalUrl, body)
	}
	return body, ContentType, nil
}
