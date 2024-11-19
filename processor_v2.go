package ninjacrawler

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/sync/errgroup"
	"sync"
	"sync/atomic"
)

func (app *Crawler) processStaticUrlsWithProxies(urls []UrlCollection, config ProcessorConfig, total *int32, crawlLimit int) bool {
	if len(urls) == 0 {
		return true
	}
	// Constants and channels
	const bufferSize = 100
	urlChan := make(chan UrlCollection, bufferSize)
	done := make(chan struct{})
	defer close(done)

	// Error group for better error handling
	g, ctx := errgroup.WithContext(context.Background())
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Atomic counters
	var (
		batchCount     atomic.Int32
		shouldContinue atomic.Bool
	)
	shouldContinue.Store(true)

	// URL producer
	g.Go(func() error {
		defer close(urlChan)
		for _, url := range urls {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case urlChan <- url:
			}
		}
		return nil
	})

	// Worker pool
	proxyPool := newProxyPool(app.engine.ProxyServers)
	for i := 0; i < app.engine.ConcurrentLimit; i++ {
		g.Go(func() error {
			return app.worker(ctx, urlChan, &batchCount, total, crawlLimit, config, proxyPool, &shouldContinue)
		})
	}

	// Wait for all goroutines to complete
	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		app.Logger.Error("Worker pool error: %v", err)
	}

	return shouldContinue.Load()
}

type ProxyPool struct {
	proxies []Proxy
	mu      sync.Mutex
	current int
}

func newProxyPool(proxies []Proxy) *ProxyPool {
	return &ProxyPool{
		proxies: proxies,
		current: 0,
	}
}

func (p *ProxyPool) getNext() Proxy {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.proxies) == 0 {
		return Proxy{}
	}

	proxy := p.proxies[p.current]
	p.current = (p.current + 1) % len(p.proxies)
	return proxy
}

func (app *Crawler) worker(
	ctx context.Context,
	urlChan <-chan UrlCollection,
	batchCount *atomic.Int32,
	total *int32,
	crawlLimit int,
	config ProcessorConfig,
	proxyPool *ProxyPool,
	shouldContinue *atomic.Bool,
) error {
	for urlCollection := range urlChan {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := app.processURL(ctx, urlCollection, batchCount, total, crawlLimit, config, proxyPool, shouldContinue); err != nil {
				if errors.Is(err, ErrCrawlLimitReached) {
					return nil
				}
				app.Logger.Error("URL processing error: %v", err)
			}
		}
	}
	return nil
}

var ErrCrawlLimitReached = errors.New("crawl limit reached")

func (app *Crawler) processURL(
	ctx context.Context,
	urlCollection UrlCollection,
	batchCount *atomic.Int32,
	total *int32,
	crawlLimit int,
	config ProcessorConfig,
	proxyPool *ProxyPool,
	shouldContinue *atomic.Bool,
) error {
	if crawlLimit > 0 && atomic.LoadInt32(total) >= int32(crawlLimit) {
		shouldContinue.Store(false)
		return ErrCrawlLimitReached
	}

	if app.shouldSkipURL(urlCollection.Url) {
		app.Logger.Debug("[SKIP] Robots disallowed: %s", urlCollection.Url)
		return nil
	}

	return app.safeCrawl(ctx, urlCollection, batchCount, total, crawlLimit, config, proxyPool, shouldContinue)
}

func (app *Crawler) safeCrawl(
	ctx context.Context,
	urlCollection UrlCollection,
	batchCount *atomic.Int32,
	total *int32,
	crawlLimit int,
	config ProcessorConfig,
	proxyPool *ProxyPool,
	shouldContinue *atomic.Bool,
) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic recovered: %v", r)
			app.HandlePanic(r)
		}
	}()

	proxy := proxyPool.getNext()
	batchCount.Add(1)

	app.openBrowsers(proxy)
	defer func() {
		app.closeBrowsers()
	}()

	if app.engine.ApplyRandomSleep != nil && *app.engine.ApplyRandomSleep {
		app.applySleep()
	}

	atomic.AddInt32(&app.ReqCount, 1)
	app.CurrentCollection = config.OriginCollection
	app.CurrentUrlCollection = urlCollection

	page := app.openPages()
	defer app.closePages(page)

	ok := app.crawlWithProxies(page, urlCollection, config, 0, proxy)
	if ok && crawlLimit > 0 {
		if atomic.AddInt32(total, 1) >= int32(crawlLimit) {
			atomic.AddInt32(total, -1)
			shouldContinue.Store(false)
			return ErrCrawlLimitReached
		}
	}

	return nil
}

func (app *Crawler) shouldSkipURL(url string) bool {
	return app.preference.CheckRobotsTxt != nil &&
		*app.preference.CheckRobotsTxt &&
		!shouldCrawl(url, app.robotsData, app.GetUserAgent())
}
