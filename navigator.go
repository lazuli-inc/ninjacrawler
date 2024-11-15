package ninjacrawler

import (
	"context"
	"strings"
	"sync/atomic"
	"time"
)

func (app *Crawler) Navigate(url string, engines ...Engine) (*NavigationContext, error) {
	app.overrideEngineDefaults(app.engine, &app.CurrentProcessorConfig.Engine)
	if len(engines) > 0 {
		eng := engines[0]
		app.overrideEngineDefaults(app.engine, &eng)
	}
	proxy := app.getCurrentProxy()
	if shouldRotateProxy {
		if len(app.engine.ProxyServers) == 0 && app.engine.ProxyStrategy == ProxyStrategyRotation {
			app.Logger.Fatal("No proxies provided for rotation")
		}
		proxyIndex := int(atomic.LoadInt32(&app.lastWorkingProxyIndex))
		proxyIndex = (proxyIndex + 1) % len(app.engine.ProxyServers)
		app.Logger.Summary("Error with proxy Retrying with proxy: %s", app.engine.ProxyServers[proxyIndex].Server)
		shouldRotateProxy = false
		proxy = app.engine.ProxyServers[proxyIndex]

		atomic.StoreInt32(&app.CurrentProxyIndex, int32(proxyIndex))
		atomic.StoreInt32(&app.lastWorkingProxyIndex, int32(proxyIndex))
	}
	page := app.openPages()

	atomic.AddInt32(&app.ReqCount, 1)

	// Freeze all goroutines after n requests
	if atomic.LoadInt32(&app.ReqCount) > 0 && atomic.LoadInt32(&app.ReqCount)%int32(app.engine.SleepAfter) == 0 {
		app.Logger.Info("Sleeping %d seconds after %d operations", app.engine.SleepDuration, app.engine.SleepAfter)
		time.Sleep(time.Duration(app.engine.SleepDuration) * time.Second)
	}

	// Add a timeout for the navigation process
	ctx, cancel := context.WithTimeout(context.Background(), app.engine.Timeout*2)
	defer cancel()
	navigationContext, err := app.navigateTo(ctx, page, url, "DeepLink", false, proxy)
	if err != nil {
		if strings.Contains(err.Error(), "StatusCode:404") {
			return nil, err
		} else if strings.Contains(err.Error(), "isRetryable") {
			// Rotate proxy if it's a retryable error
			if len(app.engine.ProxyServers) > 0 && app.engine.ProxyStrategy == ProxyStrategyRotation {
				shouldRotateProxy = true
				if app.engine.RetrySleepDuration > 0 {
					app.Logger.Info("Sleeping %d seconds before retrying", app.engine.RetrySleepDuration)
					time.Sleep(time.Duration(app.engine.RetrySleepDuration) * time.Minute)
				}
				// Retry with the next proxy and return the result
				return app.Navigate(url, engines...)
				//return nil, err
			}
			if app.engine.RetrySleepDuration > 0 {
				app.HandleThrottling(1, 0)
			}

			return nil, err
		} else {
			return nil, err
		}
	}

	defer app.closePages(page)
	return navigationContext, nil
}

func (app *Crawler) Navigates(url string, fn func(*NavigationContext) error, engines ...Engine) error {
	app.overrideEngineDefaults(app.engine, &app.CurrentProcessorConfig.Engine)
	if len(engines) > 0 {
		eng := engines[0]
		app.overrideEngineDefaults(app.engine, &eng)
	}
	proxy := app.getCurrentProxy()
	if shouldRotateProxy {
		if len(app.engine.ProxyServers) == 0 && app.engine.ProxyStrategy == ProxyStrategyRotation {
			app.Logger.Fatal("No proxies provided for rotation")
		}
		proxyIndex := int(atomic.LoadInt32(&app.lastWorkingProxyIndex))
		proxyIndex = (proxyIndex + 1) % len(app.engine.ProxyServers)
		app.Logger.Summary("Error with proxy Retrying with proxy: %s", app.engine.ProxyServers[proxyIndex].Server)
		shouldRotateProxy = false
		proxy = app.engine.ProxyServers[proxyIndex]

		atomic.StoreInt32(&app.CurrentProxyIndex, int32(proxyIndex))
		atomic.StoreInt32(&app.lastWorkingProxyIndex, int32(proxyIndex))
	}
	page := app.openPages()

	atomic.AddInt32(&app.ReqCount, 1)

	// Freeze all goroutines after n requests
	if atomic.LoadInt32(&app.ReqCount) > 0 && atomic.LoadInt32(&app.ReqCount)%int32(app.engine.SleepAfter) == 0 {
		app.Logger.Info("Sleeping %d seconds after %d operations", app.engine.SleepDuration, app.engine.SleepAfter)
		time.Sleep(time.Duration(app.engine.SleepDuration) * time.Second)
	}

	// Add a timeout for the navigation process
	ctx, cancel := context.WithTimeout(context.Background(), app.engine.Timeout*2)
	defer cancel()
	navigationContext, err := app.navigateTo(ctx, page, url, "DeepLink", false, proxy)
	if err != nil {
		if strings.Contains(err.Error(), "StatusCode:404") {
			return err
		} else if strings.Contains(err.Error(), "isRetryable") {
			// Rotate proxy if it's a retryable error
			if len(app.engine.ProxyServers) > 0 && app.engine.ProxyStrategy == ProxyStrategyRotation {
				shouldRotateProxy = true
				if app.engine.RetrySleepDuration > 0 {
					app.Logger.Info("Sleeping %d seconds before retrying", app.engine.RetrySleepDuration)
					time.Sleep(time.Duration(app.engine.RetrySleepDuration) * time.Minute)
				}
				// Retry with the next proxy and return the result
				return app.Navigates(url, fn, engines...)
				//return nil, err
			}
			if app.engine.RetrySleepDuration > 0 {
				app.HandleThrottling(1, 0)
			}

			return err
		} else {
			return err
		}
	}
	fnErr := fn(navigationContext)
	if fnErr != nil {
		return err
	}
	defer app.closePages(page)
	return nil
}
