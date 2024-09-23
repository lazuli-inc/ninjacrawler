package ninjacrawler

import "fmt"

func (app *Crawler) extract(processorConfig ProcessorConfig, ctx CrawlerContext) {
	switch v := processorConfig.Processor.(type) {
	case func(CrawlerContext) []UrlCollection:
		var collections []UrlCollection
		collections = v(ctx)

		for _, item := range collections {
			if item.Parent == "" && processorConfig.OriginCollection != baseCollection {
				app.Logger.Fatal("Missing Parent Url, Invalid OriginCollection: %v", item)
				continue
			}
		}
		app.insert(processorConfig.Entity, collections, ctx.UrlCollection.Url)
		if !processorConfig.Preference.DoNotMarkAsComplete {
			err := app.markAsComplete(ctx.UrlCollection.Url, processorConfig.OriginCollection)
			if err != nil {
				app.Logger.Error(err.Error())
				return
			}
		}
	case func(CrawlerContext, func([]UrlCollection, string)) error:
		shouldMarkAsComplete := true
		handleErr := v(ctx, func(collections []UrlCollection, currentPageUrl string) {
			for _, item := range collections {
				if item.Parent == "" && processorConfig.OriginCollection != baseCollection {
					app.Logger.Fatal("Missing Parent Url, Invalid OriginCollection: %v", item)
					continue
				}
			}
			if currentPageUrl != "" && currentPageUrl != ctx.UrlCollection.Url {
				shouldMarkAsComplete = false
				currentPageErr := app.SyncCurrentPageUrl(ctx.UrlCollection.Url, currentPageUrl, processorConfig.OriginCollection)
				if currentPageErr != nil {
					app.Logger.Fatal(currentPageErr.Error())
					return
				}
			} else {
				shouldMarkAsComplete = true
			}
			app.insert(processorConfig.Entity, collections, ctx.UrlCollection.Url)
		})
		if handleErr != nil {
			markAsError := app.MarkAsError(ctx.UrlCollection.Url, processorConfig.OriginCollection, handleErr.Error())
			if markAsError != nil {
				app.Logger.Info(markAsError.Error())
				return
			}
			app.Logger.Error(handleErr.Error())
		} else {
			if !processorConfig.Preference.DoNotMarkAsComplete && shouldMarkAsComplete {
				err := app.markAsComplete(ctx.UrlCollection.Url, processorConfig.OriginCollection)
				if err != nil {
					app.Logger.Error(err.Error())
					return
				}
			}
		}

	case UrlSelector:
		var collections []UrlCollection
		collections = app.processDocument(ctx.Document, v, ctx.UrlCollection)

		for _, item := range collections {
			if item.Parent == "" && processorConfig.OriginCollection != baseCollection {
				app.Logger.Fatal("Missing Parent Url, Invalid OriginCollection: %v", item)
				continue
			}
		}
		app.insert(processorConfig.Entity, collections, ctx.UrlCollection.Url)

		if !processorConfig.Preference.DoNotMarkAsComplete {
			err := app.markAsComplete(ctx.UrlCollection.Url, processorConfig.OriginCollection)
			if err != nil {
				app.Logger.Error(err.Error())
				return
			}
		}

	case func(CrawlerContext, func([]ProductDetailSelector, string)) error:
		shouldMarkAsComplete := true
		handleErr := v(ctx, func(collections []ProductDetailSelector, currentPageUrl string) {
			if currentPageUrl != "" && currentPageUrl != ctx.UrlCollection.Url {
				shouldMarkAsComplete = false
				currentPageErr := app.SyncCurrentPageUrl(ctx.UrlCollection.Url, currentPageUrl, processorConfig.OriginCollection)
				if currentPageErr != nil {
					app.Logger.Fatal(currentPageErr.Error())
					return
				}
			} else {
				shouldMarkAsComplete = true
			}

			for _, collection := range collections {
				scrapResult := ctx.scrapData(collection)
				err := app.validateProductDetail(scrapResult, processorConfig, ctx)
				if err != nil {
					app.Logger.Error(err.Error())
					continue
				}
			}
		})
		if handleErr != nil {
			markAsError := app.MarkAsError(ctx.UrlCollection.Url, processorConfig.OriginCollection, handleErr.Error())
			if markAsError != nil {
				app.Logger.Info(markAsError.Error())
				return
			}
			app.Logger.Error(handleErr.Error())
		} else {
			if !processorConfig.Preference.DoNotMarkAsComplete && shouldMarkAsComplete {
				err := app.markAsComplete(ctx.UrlCollection.Url, processorConfig.OriginCollection)
				if err != nil {
					app.Logger.Error(err.Error())
					return
				}
			}
		}
	case ProductDetailSelector:
		scrapResult := ctx.scrapData(processorConfig.Processor)
		err := app.validateProductDetail(scrapResult, processorConfig, ctx)
		if err != nil {
			app.Logger.Error(err.Error())
			return
		}
		if !processorConfig.Preference.DoNotMarkAsComplete {
			err := app.markAsComplete(ctx.UrlCollection.Url, processorConfig.OriginCollection)
			if err != nil {
				return
			}
		}
	case ProductDetailApi:
		scrapResult := ctx.handleProductDetailApi(processorConfig.Processor)
		err := app.validateProductDetail(scrapResult, processorConfig, ctx)
		if err != nil {
			app.Logger.Error(err.Error())
			return
		}
		if !processorConfig.Preference.DoNotMarkAsComplete {
			err := app.markAsComplete(ctx.UrlCollection.Url, processorConfig.OriginCollection)
			if err != nil {
				return
			}
		}
	default:
		app.Logger.Fatal("Unsupported processor type: %T", processorConfig.Processor)
	}

}

func (app *Crawler) validateProductDetail(res *ProductDetail, processorConfig ProcessorConfig, ctx CrawlerContext) error {
	invalidFields, unknownFields := validateRequiredFields(res, processorConfig.Preference.ValidationRules)
	if len(unknownFields) > 0 {
		return fmt.Errorf("unknown fields provided: %v", unknownFields)
	}
	if len(invalidFields) > 0 {
		msg := fmt.Sprintf("Validation failed: %v\n", invalidFields)
		html, _ := ctx.Document.Html()
		if *app.engine.IsDynamic {
			html = app.getHtmlFromPage(ctx.Page)
		}
		app.Logger.Html(html, ctx.UrlCollection.Url, msg, "validation")
		var err error
		if *app.engine.IgnoreRetryOnValidation {
			err = app.MarkAsMaxErrorAttempt(ctx.UrlCollection.Url, processorConfig.OriginCollection, msg)
		} else {
			err = app.MarkAsError(ctx.UrlCollection.Url, processorConfig.OriginCollection, msg)
		}
		if err != nil {
			return err
		}
		return fmt.Errorf(msg)
	}

	app.saveProductDetail(processorConfig.Entity, res)
	if !app.isLocalEnv {
		err := app.submitProductData(res)
		if err != nil {
			app.Logger.Fatal("Failed to submit product data to API Server: %v", err)
			err := app.MarkAsError(ctx.UrlCollection.Url, processorConfig.OriginCollection, err.Error())
			if err != nil {
				return err
			}
		}
	}
	return nil
}
