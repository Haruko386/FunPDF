package engine

import (
	"FunPDF/internal/dao"
	"FunPDF/internal/engine/translator"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

type TranslatorFactory struct {
	translatorDAO *dao.TranslatorDAO
}

func NewTranslatorFactory() *TranslatorFactory {
	return &TranslatorFactory{
		translatorDAO: &dao.TranslatorDAO{},
	}
}

// GetTranslator creates a configured translator implementation by name.
func (t *TranslatorFactory) GetTranslator(ctx context.Context, db *gorm.DB, translatorName, region string) (Translator, error) {
	if translatorName == "" {
		return nil, errors.New("translators name is required")
	}

	// get translators params from DB
	params, err := t.translatorDAO.GetTranslatorParams(ctx, db, translatorName)
	if err != nil {
		return nil, err
	}
	var param map[string]any
	err = json.Unmarshal(params, &param)
	if err != nil {
		return nil, err
	}

	url, err := TranslatorConfigURL(translatorName, region)
	if err != nil {
		return nil, err
	}

	// create translators instance by name
	switch translatorName {
	case "baidu":
		apiKey, ok := param["api_key"].(string)
		if !ok {
			return nil, errors.New("api_key is invalid")
		}
		appID, ok := param["app_id"].(string)
		if !ok {
			return nil, errors.New("app_id is invalid")
		}

		return translator.NewBaiduTranslator(apiKey, appID, url), nil
	case "deepl":
		apiKey, ok := param["api_key"].(string)
		if !ok {
			return nil, errors.New("api_key is invalid")
		}

		if region != "free" && region != "pro" {
			return nil, fmt.Errorf("region %s not supported for %s", region, translatorName)
		}

		return translator.NewDeeplTranslator(apiKey, url), nil
	case "google":
		apiKey, ok := param["api_key"].(string)
		if !ok {
			return nil, errors.New("api_key is invalid")
		}

		return translator.NewGoogleTranslator(apiKey, url), nil
	case "azure":
		apiKey, ok := param["api_key"].(string)
		if !ok {
			return nil, errors.New("api_key is invalid")
		}
		apiRegion, ok := param["region"].(string)
		if !ok {
			return nil, errors.New("region is invalid")
		}
		return translator.NewAzureTranslator(apiKey, apiRegion, url), nil
	default:
		return nil, fmt.Errorf("unsupported translators: %s", translatorName)
	}
}
