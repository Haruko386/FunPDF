package models

import (
	"FunPDF/internal/dto"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type AliyunModel struct {
	BaseModel
}

// Chat sends messages to the provider chat completion API.
func (a *AliyunModel) Chat(ctx context.Context, modelCfg *ModelConfig, chatCfg *ChatConfig, messages []Message, modelName string, sender func(*string, *string) error) (*dto.ChatResponse, error) {
	url := fmt.Sprintf("%s/%s", a.BaseURL, a.URLSuffix)

	isStream := chatCfg != nil && chatCfg.Stream != nil && *chatCfg.Stream
	reqBody := map[string]any{
		"model":    modelName,
		"messages": messages,
	}
	if chatCfg != nil {
		implementSiliconFlowConfig(chatCfg, reqBody)
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *modelCfg.APIKey))

	if isStream {
		return doStreamChat(a.HTTPClient, req, a.Name(), sender)
	}
	return doNoneStreamChat(a.HTTPClient, req, a.Name())
}

// ListModels fetches model metadata from the provider API.
func (a *AliyunModel) ListModels(ctx context.Context, modelCfg *ModelConfig) (*[]dto.ListModelsResponse, error) {
	url := fmt.Sprintf("%s/%s", a.BaseURL, a.URLSuffix)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *modelCfg.APIKey))

	client := a.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data map[string]any
	if err = json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	modelList, ok := data["data"].([]any)
	if !ok {
		modelList, ok = data["models"].([]any)
	}
	if !ok {
		return nil, fmt.Errorf(`"models" is not a list`)
	}

	var models []dto.ListModelsResponse
	for _, v := range modelList {
		item, ok := v.(map[string]any)
		if !ok {
			continue
		}
		modelName, ok := item["id"].(string)
		if !ok {
			continue
		}
		models = append(models, dto.ListModelsResponse{Name: modelName})
	}

	return &models, nil
}

// Name returns the model provider name.
func (a *AliyunModel) Name() string {
	return "Aliyun"
}
