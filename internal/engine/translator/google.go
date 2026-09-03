package translator

import (
	"FunPDF/internal/common"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/translate"
	"golang.org/x/text/language"
	"google.golang.org/api/option"
)

const GoogleTranslatorAPIReference = "https://cloud.google.com/translate/docs"

type GoogleTranslator struct {
	APIKey     string `json:"api_key"`
	URL        string `json:"url"`
	httpClient *http.Client
}

func NewGoogleTranslator(apiKey, url string) *GoogleTranslator {
	return &GoogleTranslator{
		APIKey: apiKey,
		URL:    url,
		httpClient: &http.Client{
			Timeout: time.Second * 15,
		},
	}
}

// Translate sends text to the provider and returns translated text.
func (g *GoogleTranslator) Translate(ctx context.Context, from, to, q string, params json.RawMessage) (string, error) {
	q = common.ConcatenatingStrings(q)

	client, err := translate.NewClient(ctx, option.WithAPIKey(g.APIKey))
	if err != nil {
		return "", fmt.Errorf("create translate client: %w", err)
	}
	defer client.Close()

	targetLang, err := language.Parse(to)
	if err != nil {
		return "", fmt.Errorf("language.Parse: %w", err)
	}

	var external map[string]any
	if len(params) > 0 {
		if err := json.Unmarshal(params, &external); err != nil {
			return "", fmt.Errorf("unmarshal params: %w", err)
		}
	}

	options := &translate.Options{}
	if from != "" {
		sourceLang, err := language.Parse(from)
		if err != nil {
			return "", fmt.Errorf("language.Parse: %w", err)
		}
		options = &translate.Options{
			Source: sourceLang,
			Model:  "nmt",
		}
		if external["format"] != nil {
			format, ok := external["format"].(string)
			if !ok {
				return "", fmt.Errorf("format is not a string")
			}
			options.Format = translate.Format(format)
		}
	}

	resp, err := client.Translate(ctx, []string{q}, targetLang, options)
	if err != nil {
		return "", fmt.Errorf("client.Translate error: %w", err)
	}
	if len(resp) == 0 {
		return "", fmt.Errorf("client.Translate returned empty response to text: %s", q)
	}

	return resp[0].Text, nil
}

// Healthy checks whether the translator has enough configuration to run.
func (g *GoogleTranslator) Healthy(ctx context.Context) bool {
	_, err := g.Translate(ctx, "en", "zh-CN", "hi", nil)
	common.Info(fmt.Sprintf("please read %s for more info", GoogleTranslatorAPIReference))
	return err == nil
}

// Name returns the translator provider name.
func (g *GoogleTranslator) Name() string {
	return "google"
}
