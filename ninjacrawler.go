package ninjacrawler

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"os"
	"path/filepath"
	"plugin"
	"reflect"
	"sync"
)

// CrawlerConfig holds the configuration for a single crawler
type CrawlerConfig struct {
	Name       string
	URL        string
	Engine     Engine
	Handler    Handler
	Processors []ProcessorConfig
	Preference AppPreference
}

type NinjaCrawler struct {
	Config []CrawlerConfig
	App    *Crawler
}

func NewNinjaCrawler() *NinjaCrawler {
	return &NinjaCrawler{}
}

func (ninja *NinjaCrawler) AddSite(config CrawlerConfig) *NinjaCrawler {
	ninja.Config = append(ninja.Config, config)
	return ninja
}

func (ninja *NinjaCrawler) Start() {
	var wg sync.WaitGroup
	wg.Add(len(ninja.Config))

	for _, config := range ninja.Config {
		cfg := config // Capture config variable for each goroutine
		go func(cfg CrawlerConfig) {
			defer wg.Done()
			NewCrawler(cfg.Name, cfg.URL, cfg.Engine).SetPreference(cfg.Preference).Handle(cfg.Handler)
		}(cfg)
	}

	wg.Wait()

	//StopInstanceIfRunningFromGCP()
}

func (ninja *NinjaCrawler) StartOnly(site string) {
	var wg sync.WaitGroup

	for _, config := range ninja.Config {
		if config.Name == site {
			wg.Add(1)
			cfg := config // Capture config variable for each goroutine
			go func(cfg CrawlerConfig) {
				defer wg.Done()
				NewCrawler(cfg.Name, cfg.URL, cfg.Engine).SetPreference(cfg.Preference).Handle(cfg.Handler)
			}(cfg)
		}
	}

	wg.Wait()

	//StopInstanceIfRunningFromGCP()
}
func (ninja *NinjaCrawler) StartPilot() {
	var wg sync.WaitGroup
	wg.Add(len(ninja.Config))

	for _, config := range ninja.Config {
		cfg := config // Capture config variable for each goroutine
		go func(cfg CrawlerConfig) {
			defer wg.Done()
			NewCrawler(cfg.Name, cfg.URL, cfg.Engine).AutoHandle(cfg.Processors)
		}(cfg)
	}

	wg.Wait()

	//StopInstanceIfRunningFromGCP()
}
func (ninja *NinjaCrawler) RunAutoPilot() {
	sites, err := ninja.App.LoadSites("sites.json")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	pluginMap := make(map[string]string)
	functionMap := make(map[string]interface{})
	ninjaPilot := NewNinjaCrawler()

	for _, site := range sites {
		var pkg *plugin.Plugin
		pkgOpened := false
		for i, processor := range site.Processors {
			isUrlSelector := !reflect.DeepEqual(processor.ProcessorType.UrlSelector, UrlSelector{})
			isElementSelector := !reflect.DeepEqual(processor.ProcessorType.ElementSelector, ElementSelector{})
			if processor.ProcessorType.Handle != nil || isUrlSelector {
				functionName, pluginPath, err := ninja.getFunctionNameAndPluginPath(&processor.ProcessorType, pluginMap)
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				var fnSymbol interface{}
				if fn, exists := functionMap[functionName]; exists {
					fnSymbol = fn
				} else {
					if !pkgOpened {
						pkg, err = plugin.Open(pluginPath)
						if err != nil {
							fmt.Println(err)
							os.Exit(1)
						}
						pkgOpened = true
					}

					// Look for the function by name in the package
					fnSymbol, err = pkg.Lookup(functionName)
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}

					functionMap[functionName] = fnSymbol
				}

				switch v := fnSymbol.(type) {
				case func(urlCollection UrlCollection, fullUrl string, a *goquery.Selection) (string, map[string]interface{}):
					fn, ok := fnSymbol.(func(urlCollection UrlCollection, fullUrl string, a *goquery.Selection) (string, map[string]interface{}))
					if !ok {
						fmt.Printf("Function %s has unexpected type %T\n", functionName, fnSymbol)
					}
					processor.ProcessorType.UrlSelector.Handler = fn
					site.Processors[i].Processor = processor.ProcessorType.UrlSelector
				case func(CrawlerContext) []UrlCollection:
					// root url Handler
					fn, ok := fnSymbol.(func(CrawlerContext) []UrlCollection)
					if !ok {
						fmt.Printf("Function %s has unexpected type in package\n", functionName)
					}
					site.Processors[i].Processor = fn
				case func(CrawlerContext, func([]UrlCollection, string)) error:
					// implement it
					fn, ok := fnSymbol.(func(CrawlerContext, func([]UrlCollection, string)) error)
					if !ok {
						fmt.Printf("Function %s has unexpected type in package\n", functionName)
					}
					site.Processors[i].Processor = fn
				case func(CrawlerContext, func([]ProductDetailSelector, string)) error:
					// implement it
					fn, ok := fnSymbol.(func(CrawlerContext, func([]ProductDetailSelector, string)) error)
					if !ok {
						fmt.Printf("Function %s has unexpected type in package\n", functionName)
					}
					site.Processors[i].Processor = fn
				default:
					fmt.Printf("Invalid Signature: %T\n", v)
				}
			} else if isElementSelector {
				for _, element := range processor.ProcessorType.ElementSelector.Elements {
					if element.Plugin != "" {
						ElementPluginPath := fmt.Sprintf("plugins/%s/elements", site.Name)
						pluginPath, err := ninja.buildPlugin(ElementPluginPath, element.Plugin)
						if err != nil {
							fmt.Println(err)
							os.Exit(1)
						}
						if !pkgOpened {
							pkg, err = plugin.Open(pluginPath)
							if err != nil {
								fmt.Println(err)
								os.Exit(1)
							}
							pkgOpened = true
						}

						var fnSymbol interface{}
						// Look for the function by name in the package
						fnSymbol, err = pkg.Lookup(element.Plugin)
						if err != nil {
							fmt.Println(err)
							os.Exit(1)
						}
						fmt.Printf("fnSymbol %T\n", fnSymbol)

						switch v := fnSymbol.(type) {
						case func(CrawlerContext) string:
							fn, ok := fnSymbol.(func(CrawlerContext) string)
							if !ok {
								fmt.Printf("Function %s has unexpected type in package\n", element.Plugin)
							}
							site.Processors[i].Processor = ProductDetailSelector{
								ProductName: fn,
							}
						case func(CrawlerContext) []string:
							fn, ok := fnSymbol.(func(CrawlerContext) []string)
							if !ok {
								fmt.Printf("Function %s has unexpected type in package\n", element.Plugin)
							}
							site.Processors[i].Processor = fn
						case func(CrawlerContext) []AttributeItem:
							fn, ok := fnSymbol.(func(CrawlerContext) []AttributeItem)
							if !ok {
								fmt.Printf("Function %s has unexpected type in package\n", element.Plugin)
							}
							site.Processors[i].Processor = fn

						default:
							fmt.Printf("Invalid Signature: %T\n", v)
						}
					}
				}
			}
		}

		ninjaPilot.AddSite(site)
	}

	ninjaPilot.StartPilot()
}

func (ninja *NinjaCrawler) getFunctionNameAndPluginPath(processorType *ProcessorType, pluginMap map[string]string) (string, string, error) {
	var functionName, path, filename string
	isUrlSelector := !reflect.DeepEqual(processorType.UrlSelector, UrlSelector{})
	if isUrlSelector {
		functionName = processorType.UrlSelector.Handle.FunctionName
		path = processorType.UrlSelector.Handle.Namespace
		filename = processorType.UrlSelector.Handle.Filename
	} else if processorType.Handle != nil {
		functionName = processorType.Handle.FunctionName
		path = processorType.Handle.Namespace
		filename = processorType.Handle.Filename
	}

	pluginPath, exists := pluginMap[functionName]
	if !exists {
		src := filepath.Join(path, filename)
		pluginPath = filepath.Join(path, functionName+".so")

		if err := ninja.App.generatePlugin(src, pluginPath); err != nil {
			return "", "", fmt.Errorf("failed to build plugin %s: %v", functionName, err)
		}

		pluginMap[functionName] = pluginPath
	}

	return functionName, pluginPath, nil
}

func (ninja *NinjaCrawler) buildPlugin(path, functionName string) (string, error) {
	filename := "Plugin.go"
	src := filepath.Join(path, filename)
	pluginPath := filepath.Join(path, functionName+".so")

	if err := ninja.App.generatePlugin(src, pluginPath); err != nil {
		return "", fmt.Errorf("failed to build plugin %s: %v", functionName, err)
	}
	return pluginPath, nil
}
