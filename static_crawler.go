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
	body, err := app.getResponseBody(client, urlString, proxyServer)
	if err != nil {
		return nil, err
	}

	// Create a reader that can decode the response body with the correct encoding
	reader, err := charset.NewReader(strings.NewReader(string(body)), "")
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
	body, err := app.getResponseBody(client, urlString, proxyServer)
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

func (app *Crawler) getResponseBody(client *http.Client, urlString string, proxyServer Proxy) ([]byte, error) {
	proxyIp := ""
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
	}

	if app.engine.Provider == "zenrows" {

		zenrowsApiKey := app.Config.EnvString("ZENROWS_API_KEY")
		urlString = fmt.Sprintf("https://api.zenrows.com/v1/?apikey=%s&url=%s&custom_headers=true", zenrowsApiKey, urlString)
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
		return nil, fmt.Errorf("Failed to create request: %v", err)
	}

	req.Header.Set("User-Agent", app.userAgent)
	req.Header.Set("Referer", app.BaseUrl)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Error sending request: from %s to %v", proxyIp, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("failed to fetch page: %v", resp.Status)
		app.Logger.Html(string(body), urlString, msg)
		return nil, fmt.Errorf(msg)
	}
	return body, nil
}
