package engine

import (
	"FunPDF/conf"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// loadTranslatorConfig loads a translator configuration file from disk.
func loadTranslatorConfig(name string) (json.RawMessage, error) {
	data, err := conf.ReadTranslatorConfig(name)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	return data, nil
}

// TranslatorConfigURL resolves the endpoint URL for a translator and region.
func TranslatorConfigURL(translatorName, region string) (string, error) {
	config, err := loadTranslatorConfig(translatorName)
	if err != nil {
		return "", err
	}

	var cfg map[string]any
	if err := json.Unmarshal(config, &cfg); err != nil {
		return "", err
	}

	urls, ok := cfg["url"].(map[string]any)
	if !ok {
		return "", errors.New("url config is invalid")
	}
	url, ok := urls[region].(string)
	if !ok || strings.TrimSpace(url) == "" {
		return "", errors.New("url is invalid")
	}
	return url, nil
}
