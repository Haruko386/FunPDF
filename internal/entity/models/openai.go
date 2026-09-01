package models

import (
	"FunPDF/internal/dto"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type OpenAIModel struct {
	BaseModel
}

func (o *OpenAIModel) Chat(ctx context.Context, modelCfg *ModelConfig, chatCfg *ChatConfig, messages []Message, modelName string, sender func(*string, *string) error) (*dto.ChatResponse, error) {
	url := fmt.Sprintf("%s/%s", o.BaseURL, o.URLSuffix)

	isStream := chatCfg != nil && chatCfg.Stream != nil && *chatCfg.Stream
	input := make([]map[string]any, 0, len(messages))
	var instructions strings.Builder
	for _, message := range messages {
		role := strings.TrimSpace(strings.ToLower(message.Role))
		switch role {
		case "system", "developer":
			if content, ok := message.Content.(string); ok && strings.TrimSpace(content) != "" {
				if instructions.Len() > 0 {
					instructions.WriteString("\n\n")
				}
				instructions.WriteString(content)
			}
		case "assistant":
			input = append(input, map[string]any{
				"role":    "assistant",
				"content": message.Content,
			})
		default:
			input = append(input, map[string]any{
				"role":    "user",
				"content": message.Content,
			})
		}
	}

	reqBody := map[string]any{
		"model": modelName,
		"input": input,
	}
	if instructions.Len() > 0 {
		reqBody["instructions"] = instructions.String()
	}
	if chatCfg != nil {
		if chatCfg.Stream != nil && *chatCfg.Stream {
			reqBody["stream"] = true
		}
		if chatCfg.Temperature != nil {
			reqBody["temperature"] = *chatCfg.Temperature
		}
		if chatCfg.TopP != nil {
			reqBody["top_p"] = *chatCfg.TopP
		}
		if chatCfg.MaxTokens != nil {
			reqBody["max_output_tokens"] = *chatCfg.MaxTokens
		}
		if chatCfg.Verbosity != nil {
			reqBody["text"] = map[string]any{
				"verbosity": *chatCfg.Verbosity,
			}
		}
		if chatCfg.Thinking != nil && *chatCfg.Thinking {
			effort := "medium"
			if chatCfg.Effort != nil && strings.TrimSpace(*chatCfg.Effort) != "" {
				effort = strings.TrimSpace(*chatCfg.Effort)
			}
			switch effort {
			case "default":
				effort = "medium"
			}
			reqBody["reasoning"] = map[string]any{
				"effort":  effort,
				"summary": "auto",
			}
		}
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
		client := o.HTTPClient
		if client == nil {
			client = http.DefaultClient
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("chat request failed: status=%d body=%s", resp.StatusCode, string(body))
		}

		var answer strings.Builder
		var reasonContent strings.Builder
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, ":") || strings.HasPrefix(line, "event:") {
				continue
			}
			line = strings.TrimPrefix(line, "data:")
			line = strings.TrimSpace(line)
			if line == "[DONE]" {
				break
			}

			var data map[string]any
			if err = json.Unmarshal([]byte(line), &data); err != nil {
				return nil, err
			}
			eventType, _ := data["type"].(string)
			switch eventType {
			case "response.output_text.delta":
				content, _ := data["delta"].(string)
				if content == "" {
					continue
				}
				answer.WriteString(content)
				if sender != nil {
					if err := sender(&content, nil); err != nil {
						return nil, err
					}
				}
			case "response.reasoning_summary_text.delta":
				content, _ := data["delta"].(string)
				if content == "" {
					continue
				}
				reasonContent.WriteString(content)
				if sender != nil {
					if err := sender(nil, &content); err != nil {
						return nil, err
					}
				}
			case "response.failed", "error":
				return nil, fmt.Errorf("chat request failed: %s", line)
			}
		}
		if err = scanner.Err(); err != nil {
			return nil, err
		}

		answerText := answer.String()
		reasonText := reasonContent.String()
		return &dto.ChatResponse{
			Answer:        &answerText,
			ReasonContent: &reasonText,
		}, nil
	}
	client := o.HTTPClient
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
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("chat request failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var data map[string]any
	if err = json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	var answer strings.Builder
	if outputText, ok := data["output_text"].(string); ok {
		answer.WriteString(outputText)
	}
	var reasonContent strings.Builder
	if output, ok := data["output"].([]any); ok {
		for _, itemValue := range output {
			item, ok := itemValue.(map[string]any)
			if !ok {
				continue
			}
			itemType, _ := item["type"].(string)
			if itemType == "reasoning" {
				if summary, ok := item["summary"].([]any); ok {
					for _, summaryValue := range summary {
						summaryItem, ok := summaryValue.(map[string]any)
						if !ok {
							continue
						}
						text, _ := summaryItem["text"].(string)
						reasonContent.WriteString(text)
					}
				}
				continue
			}
			content, ok := item["content"].([]any)
			if !ok {
				continue
			}
			for _, contentValue := range content {
				contentItem, ok := contentValue.(map[string]any)
				if !ok {
					continue
				}
				contentType, _ := contentItem["type"].(string)
				if contentType != "output_text" {
					continue
				}
				text, _ := contentItem["text"].(string)
				if answer.Len() == 0 {
					answer.WriteString(text)
				}
			}
		}
	}

	answerText := answer.String()
	reasonText := reasonContent.String()
	return &dto.ChatResponse{
		Answer:        &answerText,
		ReasonContent: &reasonText,
	}, nil
}

func (o *OpenAIModel) ListModels(ctx context.Context, modelCfg *ModelConfig) (*[]dto.ListModelsResponse, error) {
	url := fmt.Sprintf("%s/%s", o.BaseURL, o.URLSuffix)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *modelCfg.APIKey))

	client := o.HTTPClient
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
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("list models request failed: status=%d body=%s", resp.StatusCode, string(body))
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

func (o *OpenAIModel) Name() string {
	return "OpenAI"
}
