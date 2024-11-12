package ninjacrawler

import (
	"fmt"
	"github.com/temoto/robotstxt"
	"net/http"
	"time"
)

func (app *Crawler) bootstrap() {
	if app.preference.CheckRobotsTxt != nil && *app.preference.CheckRobotsTxt {
		app.checkRobotsTxt()
	}
}

func (app *Crawler) checkRobotsTxt() {
	app.Logger.Info("Checking robots.txt")
	robotsData, isUserAgentAllowed := checkRobotsTxt(app.BaseUrl, app.GetUserAgent())
	if isUserAgentAllowed {
		app.robotsData = robotsData
	} else {
		app.Logger.Summary("Crawling is disallowed by robots.txt")
		app.Logger.Fatal("Crawling is disallowed by robots.txt")
	}
}

func checkRobotsTxt(url, userAgent string) (*robotstxt.RobotsData, bool) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url+"/robots.txt", nil)
	if err != nil {
		return nil, true
	}
	req.Header.Set("User-Agent", userAgent)
	client.Timeout = 30 * time.Second
	response, err := client.Do(req)

	if err != nil || response.StatusCode != http.StatusOK {
		fmt.Println("Could not fetch robots.txt:", err)
		return nil, true // default to allow if robots.txt can't be fetched
	}
	defer response.Body.Close()

	robotsData, err := robotstxt.FromResponse(response)
	if err != nil {
		fmt.Println("Error parsing robots.txt:", err)
		return nil, true
	}

	// Check if the user-agent is allowed to access the site
	group := robotsData.FindGroup(userAgent)
	return robotsData, group.Test("/")
}
