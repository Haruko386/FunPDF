package translator

import (
	"FunPDF/internal/common"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	ApiVersion                  = "2026-06-06"
	AzureTranslatorAPIReference = "https://learn.microsoft.com/en-us/azure/ai-services/translator/text-translation/2026-06-06/translate-api"
)

type AzureTranslator struct {
	APIKey     string `json:"api_key"`
	Region     string `json:"region"`
	URL        string `json:"url"`
	httpClient *http.Client
}

func NewAzureTranslator(apiKey, region, url string) *AzureTranslator {
	return &AzureTranslator{
		APIKey: apiKey,
		Region: region,
		URL:    url,
		httpClient: &http.Client{
			Timeout: time.Second * 15,
		},
	}
}

// Translate sends text to the provider and returns translated text.
func (a *AzureTranslator) Translate(ctx context.Context, from, to, q string, params json.RawMessage) (string, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return "", errors.New("the text is empty")
	}

	if strings.TrimSpace(to) == "" {
		return "", errors.New("the dst language is empty")
	}

	input := map[string]any{
		"text": common.ConcatenatingStrings(q),
		"targets": []map[string]any{
			{"language": to},
		},
	}

	if from != "" {
		input["language"] = from
	}

	var external map[string]any
	if len(params) > 0 {
		if err := json.Unmarshal(params, &external); err != nil {
			return "", err
		}
	}

	if external["script"] != nil && external["script"] != "" {
		input["script"] = external["script"]
	}
	if external["textType"] != nil && external["textType"] != "" {
		input["textType"] = external["textType"]
	}

	reqBody := map[string]any{
		"inputs": []map[string]any{input},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	// do request and get response
	toCtx, cancel := context.WithTimeout(ctx, a.httpClient.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(toCtx, "POST", fmt.Sprintf("%s%s", a.URL, ApiVersion), bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Ocp-Apim-Subscription-Key", a.APIKey)
	req.Header.Set("ocp-apim-subscription-region", a.Region)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		if len(body) > 0 {
			return "", fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
		}
		return "", errors.New(resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	var result map[string]any
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", fmt.Errorf("unmarshal response body: %w", err)
	}

	values, ok := result["value"].([]any)
	if !ok {
		return "", errors.New("response body is not an array")
	}
	if len(values) == 0 {
		return "", errors.New("response body has no translations")
	}

	firstValue, ok := values[0].(map[string]any)
	if !ok {
		return "", errors.New("first translation result is invalid")
	}

	translations, ok := firstValue["translations"].([]any)
	if !ok || len(translations) == 0 {
		return "", errors.New("first translation is not exists")
	}

	firstTranslation, ok := translations[0].(map[string]any)
	if !ok {
		return "", errors.New("first translation is invalid")
	}

	translatedText, ok := firstTranslation["text"].(string)
	if !ok {
		return "", errors.New("translated text is not exists")
	}
	return translatedText, nil
}

// Healthy checks whether the translator has enough configuration to run.
func (a *AzureTranslator) Healthy(ctx context.Context) bool {
	_, err := a.Translate(ctx, "en", "zh-Hans", "hi", nil)
	common.Info(fmt.Sprintf("please read %s for more info", AzureTranslatorAPIReference))
	return err == nil
}

// Name returns the translator provider name.
func (a *AzureTranslator) Name() string {
	return "azure"
}
