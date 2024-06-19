package markt

import (
	"github.com/lazuli-inc/ninjacrawler"
	"github.com/lazuli-inc/ninjacrawler/examples/concurrent/constant"
	"github.com/playwright-community/playwright-go"
	"time"
)

func UrlHandler(crawler *ninjacrawler.Crawler) {
	crawler.Collection(constant.Categories).CrawlUrls(crawler.GetBaseCollection(), ninjacrawler.UrlSelector{
		Selector:     ".l-category-button-list__in",
		SingleResult: false,
		FindSelector: "a.c-category-button",
		Attr:         "href",
	})
	crawler.Collection(constant.Products).CrawlUrls(constant.Categories, handleProducts)
}

func handleProducts(ctx ninjacrawler.CrawlerContext) []ninjacrawler.UrlCollection {
	var urls []ninjacrawler.UrlCollection
	productLinkSelector := "a.c-text-link.u-color-text--link.c-text-link--underline"
	clickAndWaitButton(ctx.App, ".u-hidden-sp li button", ctx.Page)

	items, err := ctx.Page.Locator("ul.p-card-list-no-scroll li.p-product-card.p-product-card--large").All()
	if err != nil {
		ctx.App.Logger.Info("Error fetching items:", err)
		return urls
	}

	for i, item := range items {
		//time.Sleep(time.Second)
		err := item.Click()
		if err != nil {
			ctx.App.Logger.Error("Failed to click on Product Card: %v", err)
			continue
		}

		// Wait for the modal to open and the link to be available
		_, err = ctx.Page.WaitForSelector(productLinkSelector)
		if err != nil {
			ctx.App.Logger.Html(ctx.Page, "WaitForSelector Open Modal Timeout")
			continue
		}

		doc, err := ctx.App.GetPageDom(ctx.Page)
		if err != nil {
			ctx.App.Logger.Error("Error getting page DOM:", err)
			continue
		}

		productLink, exist := doc.Find(productLinkSelector).First().Attr("href")

		fullUrl := ctx.App.GetFullUrl(productLink)
		if !exist {
			ctx.App.Logger.Error("Failed to find product link")
		} else {
			ctx.App.Logger.Info("Saving Product Link: %s", fullUrl)
			urls = append(urls, ninjacrawler.UrlCollection{Url: fullUrl})
		}

		// Close the modal

		_, err = ctx.Page.WaitForSelector("#__next > div.l-background__wrap > div.l-background__in > div > button")
		if err != nil {
			ctx.App.Logger.Error("Timeout to Close Modal")
		}
		closeModal := ctx.Page.Locator("#__next > div.l-background__wrap > div.l-background__in > div > button")
		if closeModal != nil {
			err = closeModal.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(10000)})
			if err != nil {
				ctx.App.Logger.Html(ctx.Page, "Failed to close modal")
			}

			time.Sleep(100 * time.Millisecond)
		} else {
			ctx.App.Logger.Error("Modal close button not found.")
		}

		// Add a delay after every 50 items
		if (i+1)%50 == 0 {
			ctx.App.Logger.Info("Sleeping for 5 seconds...")
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
