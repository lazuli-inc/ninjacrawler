# NinjaCrawler

## Overview

NinjaCrawler is a Go package designed to facilitate efficient web crawling using Playwright. The package allows for the configuration of various crawling parameters, initialization of the crawling process, and handling of specific URLs and product details.



## Features

NinjaCrawler offers a robust set of features to make web crawling more efficient and customizable:

-   **Highly Maintainable and Customizable**: Easily modify and extend the crawler to fit your needs.
-   **Enhanced Developer Experience**: Focus on writing CSS selectors and custom logic. Common implementations like goroutines, logging, and bot detection are managed by this PKG.
-   **Generate Sample Data**: Create numerous sample data sets without any coding changes in the development environment.
-   **Concurrency Limiting**: Control the number of concurrent operations both globally and on a per-collection basis.
-   **Boost Crawling**: Enhance the speed of the crawling process.
-   **Resource Blocking**: Block some common and specific resources, including images, fonts, and common URLs like "www.googletagmanager.com", "google.com", "googleapis.com", and "gstatic.com" to optimize crawling.
-   **Custom Proxy Support**: Use custom proxy servers to route your requests.
-   **Easy Cookie Manipulation**: Manage cookies effortlessly.
-   **Automatic DOM Capturing**: Capture the DOM automatically during navigation errors.
-   **CSV Generation and API Submission**: Generate CSV files and submit product data to an API server.
-   **Visual Monitoring Panel**: An upcoming feature to monitor crawling progress, logs, and sample data through a visual interface.

## Installation

To install the package, you need to have Go and Playwright set up. You can install the package via `go get`:


    go get github.com/uzzalhcse/ninjacrawler


## Usage

### Creating a Crawler

To create a new crawler, use the `NewCrawler` function. This initializes the crawler with the specified name and URL. You can optionally provide custom engine settings.



```
crawler := ninjacrawler.NewCrawler("example", "https://example.com", ninjacrawler.Engine{
		IsDynamic:       false,
		DevCrawlLimit:   10,
		ConcurrentLimit: 1,
	})
```


### Starting the Crawler

The `Start` method initializes the Playwright instance and connects to the database.

    crawler.Start()

### Stopping the Crawler

The `Stop` method stops the Playwright instance and closes the database connection.


    crawler.Stop()

### Handling URLs and Products

Use the `Handle` method to define handlers for URLs and product details.

```
crawler.Handle(ninjacrawler.Handler{
    UrlHandler: func(c *ninjacrawler.Crawler) {
        // Your URL handling logic here
    },
    ProductHandler: func(c *ninjacrawler.Crawler) {
        // Your product handling logic here
    },
})
```

### Complete Example
```
func main() {
	ninjacrawler.NewCrawler("example", "https://example.com", ninjacrawler.Engine{
		BoostCrawling: true,
		IsDynamic:       false,
		DevCrawlLimit:   10,
		ConcurrentLimit: 1,
		BlockResources: true,
	}).Handle(ninjacrawler.Handler{
		UrlHandler:     aqua.UrlHandler,
		ProductHandler: aqua.ProductHandler,
	})
}

```

### UrlHandler Example
The `UrlHandler` function serves as a specific handler for the `ninjacrawler.Crawler` type. This function is designed to facilitate the crawling of URLs related to categories and products on a website, using the NinjaCrawler package.

```
crawler.Collection(ToCollection).CrawlUrls(FromCollection, urlSelector)
```
CrawlUrls Method Crawl all the urls from `FromCollection` and Save into `ToCollection`
```
func UrlHandler(crawler *ninjacrawler.Crawler) {
    // Define the selector for categories
    categorySelector := ninjacrawler.UrlSelector{
        Selector:     "ul.Header_Navigation_List_Item_Sub_Group_Inner",
        SingleResult: false,
        FindSelector: "a",
        Attr:         "href",
    }
    
    // Define the selector for products
    productSelector := ninjacrawler.UrlSelector{
        Selector:     "div.CategoryTop_Series_Item_Content_List",
        SingleResult: false,
        FindSelector: "a",
        Attr:         "href",
    }
    
    // Crawl URLs for categories and products
    crawler.Collection(constant.Categories).CrawlUrls(crawler.GetBaseCollection(), categorySelector)
    crawler.Collection(constant.Products).CrawlUrls(constant.Categories, productSelector)
}

```

**UrlHandler Function**:

-   **Parameters**: Receives a pointer to a `ninjacrawler.Crawler` instance (`crawler`), allowing manipulation of the crawler's state and operations.
-   **Category Selector**: Defines a `ninjacrawler.UrlSelector` for categories, specifying the CSS selector, nested element selector, and attribute to extract URLs.
-   **Product Selector**: Defines a `ninjacrawler.UrlSelector` for products, specifying similar details for product URL extraction.
-   **CrawlUrls Method**: Initiates crawling for URLs using the defined selectors, facilitating the retrieval of category and product URLs.

### Custom URL Handling Logic

Implement custom logic using handlers in the URL selectors. In some cases we may need to implement custom logic. by using this pkg we can easily implement our own custom logic. Lets see this example.

``` 
categorySelector := crawler.UrlSelector{
		Selector:     "div.index.clearfix ul.clearfix li",
		SingleResult: false,
		FindSelector: "a",
		Attr:         "href",
		Handler:      customHandler,
	}
	categoryProductSelector := crawler.UrlSelector{
		Selector:     "div.index.clearfix ul.clearfix li",
		SingleResult: false,
		FindSelector: "a",
		Attr:         "href",
		Handler: func(collection crawler.UrlCollection, fullUrl string, a *goquery.Selection) (string, map[string]interface{}) {
			if strings.Contains(fullUrl, "/tool/product/") {
				return fullUrl, nil
			}
			return "", nil
		},
	}


```
The handler function signature:

    func(urlCollection UrlCollection, fullUrl string, a *goquery.Selection) (string, map[string]interface{})

Return metadata if needed: `map[string]interface{}`
```
subCategorySelector := crawler.UrlSelector{
		Selector:     "ul.category_list li",
		SingleResult: false,
		FindSelector: "div.category_order h4 a",
		Attr:         "href",
		Handler: func(collection crawler.UrlCollection, fullUrl string, a *goquery.Selection) (string, map[string]interface{}) {
			brand1 := a.Text()
			return fullUrl, map[string]any{
				"brand1": brand1,
			}
		},
	}
```
### Custom Logic Example

Implement unique custom logic:
```

func categoryHandler(document *goquery.Document, urlCollection *crawler.UrlCollection, page playwright.Page) []crawler.UrlCollection {

	categoryUrls := []crawler.UrlCollection{}
	categoryDiv := document.Find("#menuList > div.submenu > div.accordion > dl")

	// Get the total number of items in the selection
	totalCats := categoryDiv.Length()

	// Iterate over all items except the last three
	categoryDiv.Slice(0, totalCats-3).Each(func(i int, cat *goquery.Selection) {
		cat.Find("dd > div > ul > li ul > li ul > li").Each(func(j int, lMain *goquery.Selection) {
			if href, ok := lMain.Find("a").Attr("href"); ok {
				fullUrl := crawler.App.BaseUrl + href
				categoryUrls = append(categoryUrls, crawler.UrlCollection{
					Url:      fullUrl,
					MetaData: nil,
				})
			} else {
				crawler.App.Logger.Error("Category URL not found.")
			}
		})
	})

	return categoryUrls
}
```

### Custom Engine Configuration

Configure custom engine settings for specific collections:
In some cases we may need to configure our custom engine configure  for specific collection. For Example We Want to Set `ConcurrentLimit = 5` and Block Common Resources for `Products` Collection. Then We just have to do like this example .

    crawler.Collection(constant.Products).SetConcurrentLimit(5).SetBlockResources(true).CrawlUrls(constant.Categories, productSelector)

This custom configuration will overwrite the global engine configuration.

Here is the Available Engine Configuration:

### Engine Structure

-   **BrowserType**: Type of browser to use (e.g., "chromium").
-   **ConcurrentLimit**: Limit on concurrent operations.
-   **IsDynamic**: Indicates if the crawling is dynamic.
-   **DevCrawlLimit**: Limit for development crawling.
-   **BlockResources**: Indicates if resource blocking is enabled. When enabled, it blocks common resources like images, fonts, and specific URLs such as "[www.googletagmanager.com](http://www.googletagmanager.com)", "google.com", "googleapis.com", and "gstatic.com".
-   **BlockedURLs**: List of URLs to block.
-   **BoostCrawling**: Indicates if crawling should be boosted.
-   **ProxyServers**: List of proxy servers.
-   **CookieConsent**: Cookie consent settings.


### Cookie Consent Handling

Handle cookie consent automatically:
Lets use Another configuration like CookieConsent.Assume target site require cookie must enabled. In that case we dont need to manually inspect to find out which cookie value actually we need to set and althow its a time-consuming and tricky.

![Screenshot from 2024-06-13 19-12-59](https://github.com/uzzalhcse/crawl-test/assets/42272402/91f8b138-9d92-45b0-af5f-cc4da1a4bc5c)

Using this pkg we can handle it just mentioning  the action button name . in that case `Accept Cookies`. After Accept we may need to wait sometimes to process further . Lets implement it.

### Global Configuration
```
crawler := ninjacrawler.NewCrawler("example", "https://example.com", ninjacrawler.Engine{
		IsDynamic:       false,
		DevCrawlLimit:   10,
		ConcurrentLimit: 1,
		CookieConsent: &ninjacrawler.CookieAction{
			ButtonText:       "Accept Cookies",
			SleepAfterAction: 5,
		},
	})
```
### Specific Collection Configuration

```
crawler.Collection(constant.Categories).SetCookieConsent(&ninjacrawler.CookieAction{
		ButtonText:       "Accept Cookies",
		SleepAfterAction: 5,
	},).CrawlUrls(crawler.GetBaseCollection(), categorySelector)
```

**NB**: Some sites require user inputs to accept cookie like name, age, etc. Current version is not support yet but this feature will include soon . 

## ProductHandler
The ProductHandler function configures how product details should be extracted from crawled pages:

````
func ProductHandler(crawler *ninjacrawler.Crawler) {
    crawler.ProductDetailSelector = ninjacrawler.ProductDetailSelector{
        Jan: "",
        PageTitle: &ninjacrawler.SingleSelector{
            Selector: "title",
        },
        Url: getUrlHandler,
        Images: &ninjacrawler.MultiSelectors{
            Selectors: []ninjacrawler.Selector{
                {Query: ".details .intro .image img", Attr: "src"},
            },
        },
        ProductCodes:     func(document *goquery.Document, urlCollection ninjacrawler.UrlCollection) []string { return []string{} },
        Maker:            "",
        Brand:            "",
        ProductName:      productNameHandler,
        Category:         getProductCategory,
        Description:      getProductDescription,
        Reviews:          func(document *goquery.Document, urlCollection ninjacrawler.UrlCollection) []string { return []string{} },
        ItemTypes:        func(document *goquery.Document, urlCollection ninjacrawler.UrlCollection) []string { return []string{} },
        ItemSizes:        func(document *goquery.Document, urlCollection ninjacrawler.UrlCollection) []string { return []string{} },
        ItemWeights:      func(document *goquery.Document, urlCollection ninjacrawler.UrlCollection) []string { return []string{} },
        SingleItemSize:   "",
        SingleItemWeight: "",
        NumOfItems:       "",
        ListPrice:        "",
        SellingPrice:     "",
        Attributes:       getProductAttribute,
    }
    crawler.Collection(constant.ProductDetails).CrawlPageDetail(constant.Products)
}

````

**ProductHandler Function**:
This function processes each field in `ProductDetailSelector` and applies the appropriate handler based on the field type.
-   **Parameters**: Receives a pointer to a `ninjacrawler.Crawler` instance (`crawler`), configuring how product details should be extracted from crawled pages.
-   **Product Detail Selectors**: Specifies how various product details should be extracted, including page title, URL, images, product name, category, description, and attributes.
-   **CrawlPageDetail Method**: Crawls and extracts detailed product information from the specified product pages.

**Some Example of PageHandler**:
Examples of specific handlers for product details:

```
func productNameHandler(document *goquery.Document, urlCollection ninjacrawler.UrlCollection) string {
    return strings.Trim(document.Find(".details .intro h2").First().Text(), " \n")
}

func getUrlHandler(document *goquery.Document, urlCollection ninjacrawler.UrlCollection) string {
    return urlCollection.Url
}

func getProductCategory(document *goquery.Document, urlCollection ninjacrawler.UrlCollection) string {
    categoryItems := make([]string, 0)
    document.Find("ol.st-Breadcrumb_List li.st-Breadcrumb_Item").Each(func(i int, s *goquery.Selection) {
        if i >= 2 {
            txt := strings.TrimSpace(s.Text())
            categoryItems = append(categoryItems, txt)
        }
    })
    return strings.Join(categoryItems, " > ")
}
```