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

const deeplAPIReference = "https://developers.deepl.com/docs/getting-started/quickstart"

type DeeplTranslator struct {
	APIKey     string `json:"api_key"`
	URL        string `json:"url"`
	httpClient *http.Client
}

type DeeplConfig struct {
	Name string `json:"name"`
	URL  struct {
		Free string `json:"free"`
		Pro  string `json:"pro"`
	} `json:"url"`
}

func NewDeeplTranslator(apiKey, url string) *DeeplTranslator {
	return &DeeplTranslator{
		APIKey: apiKey,
		URL:    url,
		httpClient: &http.Client{
			Timeout: time.Second * 15,
		},
	}
}

// Translate for deepl, it receives `string[]` for translate. our APP only translate a sentence once. So we use `string[1]`
func (d *DeeplTranslator) Translate(ctx context.Context, from, to, q string, params json.RawMessage) (string, error) {
	// build reqBody
	reqBody := make(map[string]any)

	q = strings.TrimSpace(q)
	if q == "" {
		return "", errors.New("the text is empty")
	}
	reqBody["text"] = []string{common.ConcatenatingStrings(q)}

	if from != "" {
		reqBody["source_lang"] = from
	}

	if to != "" {
		reqBody["target_lang"] = to
	} else {
		return "", errors.New("the dst language should not be empty")
	}

	var external map[string]any
	if len(params) > 0 {
		if err := json.Unmarshal(params, &external); err != nil {
			return "", err
		}
	}
	if external["model_type"] != nil && external["model_type"].(string) != "" {
		mType := external["model_type"].(string)
		if mType == "quality_optimized" || mType == "prefer_quality_optimized" || mType == "latency_optimized" {
			reqBody["model_type"] = mType
		}
	}

	if external["formality"] != nil && external["formality"].(string) != "" {
		formality := external["formality"].(string)
		switch formality {
		case "default", "more", "less", "prefer_more", "prefer_less":
			reqBody["formality"] = formality
		default:
		}
	}

	if external["preserve_formatting"] != nil && external["preserve_formatting"].(bool) {
		reqBody["preserve_formatting"] = true
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	// do request and get response
	toCtx, cancel := context.WithTimeout(ctx, d.httpClient.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(toCtx, "POST", d.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "DeepL-Auth-Key "+d.APIKey)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode != 200 {
		resp.Body.Close()
		return "", errors.New(resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", fmt.Errorf("unmarshal response body: %w", err)
	}

	translations, ok := result["translations"].([]any)
	if !ok {
		return "", errors.New(resp.Status)
	}
	firstTranslation, ok := translations[0].(map[string]any)
	if !ok {
		return "", errors.New(fmt.Sprint("translations is not exist"))
	}

	dst, ok := firstTranslation["text"].(string)
	if !ok {
		return "", errors.New(fmt.Sprint("translations is nil"))
	}

	return dst, nil
}

// Healthy checks whether the translator has enough configuration to run.
func (d *DeeplTranslator) Healthy(ctx context.Context) bool {
	_, err := d.Translate(ctx, "en", "zh", "hi", nil)
	common.Info(fmt.Sprintf("please read %s for more info", deeplAPIReference))
	return err == nil
}

// Name returns the translator provider name.
func (d *DeeplTranslator) Name() string {
	return "deepl"
}
