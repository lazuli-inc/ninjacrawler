package ninjacrawler

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"os"
	"path/filepath"
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
		for i, processor := range site.Processors {
			isUrlSelector := !reflect.DeepEqual(processor.ProcessorType.UrlSelector, UrlSelector{})
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
					fnSymbol, err = loadHandler(pluginPath, functionName)
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}
					functionMap[functionName] = fnSymbol
				}

				if isUrlSelector {
					fn, ok := fnSymbol.(func(urlCollection UrlCollection, fullUrl string, a *goquery.Selection) (string, map[string]interface{}))
					if !ok {
						fmt.Printf("Function %s has unexpected type in package\n", functionName)
					}
					processor.ProcessorType.UrlSelector.Handler = fn
					site.Processors[i].Processor = processor.ProcessorType.UrlSelector
				} else {
					fn, ok := fnSymbol.(func(CrawlerContext) []UrlCollection)
					if !ok {
						fmt.Printf("Function %s has unexpected type in package\n", functionName)
					}
					site.Processors[i].Processor = fn
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
