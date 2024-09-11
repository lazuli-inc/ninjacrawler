package ninjacrawler

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/charset"
	"golang.org/x/net/proxy"
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
		Timeout: (app.engine.Timeout / 1000) * time.Second,
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

	ContentType := ""
	proxyIp := ""
	urlString = app.GetQueryEscapeFullUrl(urlString)

	httpTransport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := net.Dial(network, addr)
			if err == nil {
				proxyIp = conn.RemoteAddr().String()
				if app.engine.Provider == "zenrows" || strings.Contains(proxyServer.Server, "proxy.zenrows.com") {
					app.Logger.Info("Proxy IP address: %s => %s", proxyIp, urlString)
				}
			}
			return conn, err
		},
		//DialContext: (&net.Dialer{
		//	Timeout:   client.Timeout, // Connection timeout
		//	KeepAlive: client.Timeout,
		//}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second, // TLS handshake timeout
	}

	if app.engine.Provider == "zenrows" {

		zenrowsApiKey := app.Config.EnvString("ZENROWS_API_KEY")
		queryString := app.BuildQueryString()
		urlString = fmt.Sprintf("https://api.zenrows.com/v1/?apikey=%s&url=%s&%s", zenrowsApiKey, urlString, queryString)

	} else {
		if len(app.engine.ProxyServers) > 0 {
			proxyURL, err := url.Parse(proxyServer.Server)
			if err != nil {
				log.Fatalf("Failed to parse proxy URL: %v", err)
			}
			if proxyServer.Username != "" && proxyServer.Password != "" {
				proxyURL.User = url.UserPassword(proxyServer.Username, proxyServer.Password)
				dialer, err := proxy.FromURL(proxyURL, proxy.Direct)
				if err != nil {
					log.Fatalf("Failed to obtain proxy dialer: %v", err)
				}
				httpTransport.Dial = dialer.Dial
			} else {
				httpTransport.Proxy = http.ProxyURL(proxyURL)
			}

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

	req.Header.Set("User-Agent", app.userAgent)
	req.Header.Set("Referer", app.BaseUrl)

	resp, err := client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "Client.Timeout") {
			_ = app.updateStatusCode(urlString, 408)
		}
		return nil, ContentType, fmt.Errorf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ContentType, fmt.Errorf("failed to read response body: %w", err)
	}
	ContentType = resp.Header.Get("Content-Type")
	_ = app.updateStatusCode(urlString, resp.StatusCode)
	app.CurrentUrl = resp.Request.URL.String()
	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("failed to fetch page: StatusCode:%v and Status:%v", resp.StatusCode, resp.Status)
		if resp.StatusCode == 404 {
			_ = app.MarkAsMaxErrorAttempt(urlString, app.CurrentCollection, "Url Not Found")
			return nil, ContentType, fmt.Errorf("Url Not Found StatusCode: %v", resp.StatusCode)
		}
		if app.engine.RetrySleepDuration > 0 && (resp.StatusCode == 403) {
			app.Logger.Error("failed: StatusCode:%v and Status:%v", resp.StatusCode, resp.Status)
			app.Logger.Debug("Got Blocked at URL: %s Error: %v\n", app.CurrentUrl, msg)
			app.HandleThrottling(1, resp.StatusCode)
		} else {
			app.Logger.Debug("Http Error URL: %s Error: %v\n", app.CurrentUrl, msg)
		}
		app.Logger.Html(string(body), urlString, msg)
		var jsonResponse map[string]interface{}
		err = json.Unmarshal(body, &jsonResponse)

		if err == nil && jsonResponse["code"] == "RESP001" && jsonResponse["status"] == 422 && strings.Contains(jsonResponse["title"].(string), "Could not get content. try enabling premium proxies for a higher success rate (RESP001)") {

			if attempt <= app.engine.MaxRetryAttempts && app.engine.ProviderOption.UsePremiumProxyRetry {
				attempt++
				fmt.Println("Zenrows response: ", jsonResponse)
				urlString += "&premium_proxy=true&proxy_country=jp"
				app.Logger.Warn("retrying with premium proxy: %s", urlString)
				body, ContentType, err = app.getResponseBody(client, urlString, proxyServer, attempt)
				if err != nil {
					return nil, ContentType, err
				}
				return body, ContentType, nil
			}
		}
		return nil, ContentType, fmt.Errorf(msg)
	}
	return body, ContentType, nil
}
