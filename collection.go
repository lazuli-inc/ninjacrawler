package ninjacrawler

import "time"

type SiteCollection struct {
	Url       string     `json:"url" bson:"url"`
	BaseUrl   string     `json:"base_url" bson:"base_url"`
	Pid       bool       `json:"pid" bson:"pid"`
	Status    bool       `json:"status" bson:"status"`
	Attempts  int        `json:"attempts" bson:"attempts"`
	StartedAt time.Time  `json:"started_at" bson:"started_at"`
	EndedAt   *time.Time `json:"ended_at" bson:"ended_at"`
}
type UrlCollection struct {
	Url       string                 `json:"url" bson:"url"`
	Parent    string                 `json:"parent" bson:"parent"`
	Status    bool                   `json:"status" bson:"status"`
	Error     bool                   `json:"error" bson:"error"`
	Attempts  int                    `json:"attempts" bson:"attempts"`
	MetaData  map[string]interface{} `json:"meta_data" bson:"meta_data"`
	CreatedAt time.Time              `json:"created_at" bson:"created_at"`
	UpdatedAt *time.Time             `json:"updated_at" bson:"updated_at"`
}

type ProductDetail struct {
	Jan              string          `json:"jan" bson:"jan"`
	PageTitle        string          `json:"page_title" bson:"page_title"`
	Url              string          `json:"url" bson:"url"`
	Images           []string        `json:"images" bson:"images"`
	ProductCodes     []string        `json:"product_codes" bson:"product_codes"`
	Maker            string          `json:"maker" bson:"maker"`
	Brand            string          `json:"brand" bson:"brand"`
	ProductName      string          `json:"product_name" bson:"product_name"`
	Category         string          `json:"category" bson:"category"`
	Description      string          `json:"description" bson:"description"`
	Reviews          []string        `json:"reviews" bson:"reviews"`
	ItemTypes        []string        `json:"item_types" bson:"item_types"`
	ItemSizes        []string        `json:"item_sizes" bson:"item_sizes"`
	ItemWeights      []string        `json:"item_weights" bson:"item_weights"`
	SingleItemSize   string          `json:"single_item_size" bson:"single_item_size"`
	SingleItemWeight string          `json:"single_item_weight" bson:"single_item_weight"`
	NumOfItems       string          `json:"num_of_items" bson:"num_of_items"`
	ListPrice        string          `json:"list_price" bson:"list_price"`
	SellingPrice     string          `json:"selling_price" bson:"selling_price"`
	Attributes       []AttributeItem `json:"attributes" bson:"attributes"`
}
type AttributeItem struct {
	Key   string `json:"key" bson:"key"`
	Value string `json:"value" bson:"value"`
}
