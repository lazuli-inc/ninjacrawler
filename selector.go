package ninjacrawler

import "github.com/PuerkitoBio/goquery"

type UrlSelector struct {
	Selector       string `json:"selector"`
	SingleResult   bool   `json:"single_result"`
	FindSelector   string `json:"find_selector"`
	Attr           string `json:"attr"`
	ToCollection   string `json:"to_collection"`
	FromCollection string `json:"from_collection"`
	Handler        func(urlCollection UrlCollection, fullUrl string, a *goquery.Selection) (string, map[string]interface{})
	Handle         *Handle `json:"handle"`
}
type Handle struct {
	Namespace    string `json:"namespace"`
	Filename     string `json:"filename"`
	FunctionName string `json:"function_name"`
}
type SingleSelector struct {
	Selector string
}

type Selector struct {
	Query        string // CSS selector
	Attr         string // Attribute to extract (e.g., "src" or "href")
	SingleResult bool
}

type MultiSelectors struct {
	Selectors     []Selector // Array of selectors
	ExcludeString []string
	IsUnique      bool
}

type ProductDetailSelector struct {
	Jan              interface{}
	PageTitle        interface{}
	Url              interface{}
	Images           interface{}
	ProductCodes     interface{}
	Maker            interface{}
	Brand            interface{}
	ProductName      interface{}
	Category         interface{}
	Description      interface{}
	Reviews          interface{}
	ItemTypes        interface{}
	ItemSizes        interface{}
	ItemWeights      interface{}
	SingleItemSize   interface{}
	SingleItemWeight interface{}
	NumOfItems       interface{}
	ListPrice        interface{}
	SellingPrice     interface{}
	Attributes       interface{}
}

type ProductDetailApi struct {
	Jan              interface{}
	PageTitle        interface{}
	Url              interface{}
	Images           interface{}
	ProductCodes     interface{}
	Maker            interface{}
	Brand            interface{}
	ProductName      interface{}
	Category         interface{}
	Description      interface{}
	Reviews          interface{}
	ItemTypes        interface{}
	ItemSizes        interface{}
	ItemWeights      interface{}
	SingleItemSize   interface{}
	SingleItemWeight interface{}
	NumOfItems       interface{}
	ListPrice        interface{}
	SellingPrice     interface{}
	Attributes       interface{}
}
