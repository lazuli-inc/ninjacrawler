package markt

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/lazuli-inc/ninjacrawler"
	"github.com/lazuli-inc/ninjacrawler/examples/concurrent/constant"
	"github.com/playwright-community/playwright-go"
	"time"
)

func UrlHandler(crawler *ninjacrawler.Crawler) {
	crawler.Collection(constant.Categories).CrawlUrls(crawler.GetBaseCollection(), ninjacrawler.UrlSelector{
		Selector:       ".l-category-button-list__in",
		SingleResult:   false,
		FindSelector:   "a.c-category-button",
		Attr:           "href",
		ToCollection:   constant.Categories,
		FromCollection: crawler.GetBaseCollection(),
	})
	crawler.Collection(constant.Products).CrawlUrls(constant.Categories, func(document *goquery.Document, collection *ninjacrawler.UrlCollection, page playwright.Page) []ninjacrawler.UrlCollection {
		return handleProducts(crawler, document, collection, page)
	})
}

func handleProducts(crawler *ninjacrawler.Crawler, document *goquery.Document, collection *ninjacrawler.UrlCollection, page playwright.Page) []ninjacrawler.UrlCollection {
	var urls []ninjacrawler.UrlCollection
	productLinkSelector := "a.c-text-link.u-color-text--link.c-text-link--underline"
	clickAndWaitButton(crawler, ".u-hidden-sp li button", page)

	items, err := page.Locator("ul.p-card-list-no-scroll li.p-product-card.p-product-card--large").All()
	if err != nil {
		crawler.Logger.Info("Error fetching items:", err)
		return urls
	}

	for i, item := range items {
		//time.Sleep(time.Second)
		err := item.Click()
		if err != nil {
			crawler.Logger.Error("Failed to click on Product Card: %v", err)
			continue
		}

		// Wait for the modal to open and the link to be available
		_, err = page.WaitForSelector(productLinkSelector)
		if err != nil {
			crawler.Logger.Html(page, "WaitForSelector Open Modal Timeout")
			continue
		}

		doc, err := crawler.GetPageDom(page)
		if err != nil {
			crawler.Logger.Error("Error getting page DOM:", err)
			continue
		}

		productLink, exist := doc.Find(productLinkSelector).First().Attr("href")

		fullUrl := crawler.GetFullUrl(productLink)
		if !exist {
			crawler.Logger.Error("Failed to find product link")
		} else {
			crawler.Logger.Info("Saving Product Link: %s", fullUrl)
			urls = append(urls, ninjacrawler.UrlCollection{Url: fullUrl})
		}

		// Close the modal

		_, err = page.WaitForSelector("#__next > div.l-background__wrap > div.l-background__in > div > button")
		if err != nil {
			crawler.Logger.Error("Timeout to Close Modal")
		}
		closeModal := page.Locator("#__next > div.l-background__wrap > div.l-background__in > div > button")
		if closeModal != nil {
			err = closeModal.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(10000)})
			if err != nil {
				crawler.Logger.Html(page, "Failed to close modal")
			}

			time.Sleep(100 * time.Millisecond)
		} else {
			crawler.Logger.Error("Modal close button not found.")
		}

		// Add a delay after every 50 items
		if (i+1)%50 == 0 {
			crawler.Logger.Info("Sleeping for 5 seconds...")
			time.Sleep(5 * time.Second)
		}

	}

	return urls
}
func clickAndWaitButton(crawler *ninjacrawler.Crawler, selector string, page playwright.Page) {
	for {
		button := page.Locator(selector)
		err := button.Click()
		page.WaitForSelector(selector, playwright.PageWaitForSelectorOptions{Timeout: playwright.Float(1000)})
		if err != nil {
			crawler.Logger.Info("No more button available")
			break
		}
	}
}
