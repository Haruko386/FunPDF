package models

import (
	"FunPDF/internal/dto"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func doStreamChat(client *http.Client, req *http.Request, providerName string, sender func(*string, *string) error) (*dto.ChatResponse, error) {
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
		if line == "" || strings.HasPrefix(line, ":") {
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
		choices, ok := data["choices"].([]any)
		if !ok || len(choices) == 0 {
			continue
		}
		choice, ok := choices[0].(map[string]any)
		if !ok {
			continue
		}
		delta, ok := choice["delta"].(map[string]any)
		if !ok {
			continue
		}
		if content, ok := delta["content"].(string); ok {
			answer.WriteString(content)
			if sender != nil {
				if err := sender(&content, nil); err != nil {
					return nil, err
				}
			}
		}

		reasoningFiled := ""
		switch providerName {
		case "DeepSeek", "MoonShot":
			reasoningFiled = "reasoning_content"
		}

		if content, ok := delta[reasoningFiled].(string); ok {
			reasonContent.WriteString(content)
			if sender != nil {
				if err := sender(nil, &content); err != nil {
					return nil, err
				}
			}
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

func doNoneStreamChat(client *http.Client, req *http.Request, providerName string) (*dto.ChatResponse, error) {
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
	choices, ok := data["choices"].([]any)
	if !ok || len(choices) == 0 {
		return nil, fmt.Errorf(`"choices" is not a list`)
	}
	choice, ok := choices[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf(`"choices[0]" is invalid`)
	}
	message, ok := choice["message"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf(`"message" is invalid`)
	}

	reasoningFiled := ""
	switch providerName {
	case "DeepSeek", "MoonShot", "SiliconFlow, Aliyun":
		reasoningFiled = "reasoning_content"
	}

	answerText, _ := message["content"].(string)
	reasonText, _ := message[reasoningFiled].(string)
	return &dto.ChatResponse{
		Answer:        &answerText,
		ReasonContent: &reasonText,
	}, nil
}

// implementDeepSeekChatConfig write chat config to req body with deepseek API reference
func implementDeepSeekChatConfig(chatCfg *ChatConfig, reqBody map[string]any) {
	if chatCfg.Stream != nil && *chatCfg.Stream {
		reqBody["stream"] = true
	}

	if chatCfg.Thinking != nil && *chatCfg.Thinking {
		var thinkingFlag string
		effort := "high"
		if chatCfg.Effort != nil {
			effort = *chatCfg.Effort
		}
		switch effort {
		case "none":
			thinkingFlag = "disabled"
		case "low":
			thinkingFlag = "disabled"
		case "medium":
			thinkingFlag = "disabled"
		case "high":
			thinkingFlag = "enabled"
			reqBody["reasoning_effort"] = "high"
		case "default":
			thinkingFlag = "enabled"
			reqBody["reasoning_effort"] = "high"
		case "max":
			thinkingFlag = "enabled"
			reqBody["reasoning_effort"] = "max"
		}
		reqBody["thinking"] = map[string]interface{}{
			"type": thinkingFlag,
		}
	} else {
		reqBody["thinking"] = map[string]interface{}{
			"type": "disabled",
		}
	}

	if chatCfg.Temperature != nil {
		reqBody["temperature"] = *chatCfg.Temperature
	}

	if chatCfg.TopP != nil {
		reqBody["top_p"] = *chatCfg.TopP
	}

	if chatCfg.MaxTokens != nil {
		reqBody["max_tokens"] = *chatCfg.MaxTokens
	}

}

func implementMoonShotConfig(chatCfg *ChatConfig, reqBody map[string]any) {
	if chatCfg.Stream != nil && *chatCfg.Stream {
		reqBody["stream"] = true
	}

	if chatCfg.Thinking != nil && *chatCfg.Thinking {
		reqBody["thinking"] = true
	} else {
		reqBody["thinking"] = false
	}

	if chatCfg.Effort != nil {
		reqBody["reasoning_effort"] = *chatCfg.Effort
	}

	if chatCfg.Temperature != nil {
		reqBody["temperature"] = *chatCfg.Temperature
	}

	if chatCfg.TopP != nil {
		reqBody["top_p"] = *chatCfg.TopP
	}

	if chatCfg.MaxTokens != nil {
		reqBody["max_tokens"] = *chatCfg.MaxTokens
	}
}

func implementSiliconFlowConfig(chatCfg *ChatConfig, reqBody map[string]any) {
	if chatCfg.Stream != nil && *chatCfg.Stream {
		reqBody["stream"] = true
	}

	if chatCfg.Thinking != nil && *chatCfg.Thinking {
		reqBody["enable_thinking"] = true
	} else {
		reqBody["enable_thinking"] = false
	}

	if chatCfg.Temperature != nil {
		reqBody["temperature"] = *chatCfg.Temperature
	}

	if chatCfg.TopP != nil {
		reqBody["top_p"] = *chatCfg.TopP
	}

	if chatCfg.MaxTokens != nil {
		reqBody["max_tokens"] = *chatCfg.MaxTokens
	}
}
