package handler

import (
	"FunPDF/internal/dto"
	"FunPDF/internal/entity/models"
	"FunPDF/internal/service"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type ModelHandler struct {
	modelSvr *service.ModelService
}

func NewModelHandler() *ModelHandler {
	return &ModelHandler{
		modelSvr: service.NewModelService(),
	}
}

// ListProviderModel list provider's model stored in DB
func (h *ModelHandler) ListProviderModel(c *gin.Context) {
	providerID := strings.TrimSpace(c.Param("provider_id"))
	if providerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "provider id is empty",
		})
		return
	}

	list, err := h.modelSvr.ListProviderModel(c.Request.Context(), providerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"data":    list,
		"message": "success",
	})
}

// SaveProviderModels save some models to DB
func (h *ModelHandler) SaveProviderModels(c *gin.Context) {
	providerID := strings.TrimSpace(c.Param("provider_id"))
	if providerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "provider id is empty",
		})
		return
	}

	var req dto.SaveModelsRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": err.Error(),
		})
		return
	}

	if req.Names == nil || len(*req.Names) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "names is empty",
		})
		return
	}

	list, err := h.modelSvr.SaveProviderModels(c.Request.Context(), providerID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"data":    list,
		"message": "success",
	})
}

// DeleteProviderModels handles deleting provider model selections.
func (h *ModelHandler) DeleteProviderModels(c *gin.Context) {
	providerID := strings.TrimSpace(c.Param("provider_id"))
	if providerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "provider id is empty",
		})
		return
	}

	var req dto.DeleteModelsRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": err.Error(),
		})
		return
	}

	if req.IDs == nil || len(*req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "ids is empty",
		})
		return
	}

	err := h.modelSvr.DeleteProviderModels(c.Request.Context(), providerID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	c.Status(http.StatusNoContent)
}

// ChatToModel chat to model
func (h *ModelHandler) ChatToModel(c *gin.Context) {
	providerID := strings.TrimSpace(c.Param("provider_id"))
	if providerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "provider id is empty",
		})
		return
	}

	var req dto.ChatRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": err.Error(),
		})
		return
	}

	modelCfg := models.ModelConfig{
		APIKey: nil,
		Region: nil,
	}

	chatCfg := models.ChatConfig{
		Stream:      req.Stream,
		Thinking:    req.Thinking,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Effort:      req.Effort,
		Verbosity:   req.Verbosity,
	}

	if req.Stream != nil && *req.Stream {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Writer.WriteHeader(http.StatusOK)
		c.Writer.Flush()

		// Create sender function that writes directly to response
		sender := func(content, reasoningContent *string) error {
			// Check for [DONE] marker (OpenAI compatible)
			if content != nil {
				if *content == "[DONE]" {
					c.SSEvent("done", "[DONE]")
					return nil
				}
				message := fmt.Sprintf("[MESSAGE]%s", *content)
				c.SSEvent("message", message)
				c.Writer.Flush()
			}

			if reasoningContent != nil {
				message := fmt.Sprintf("[REASONING]%s", *reasoningContent)
				c.SSEvent("message", message)
				c.Writer.Flush()
			}

			//logger.Info(data)
			return nil
		}

		// Convert []map[string]any to []models.Message
		messages := make([]models.Message, len(req.Messages))
		for i, msg := range req.Messages {
			role, _ := msg["role"].(string)
			content := msg["content"]
			messages[i] = models.Message{Role: role, Content: content}
		}

		if err := h.modelSvr.ChatToModelStreamWithSender(c.Request.Context(), providerID, req.ModelName, req.ModelID, messages, modelCfg, chatCfg, sender); err != nil {
			c.SSEvent("error", err.Error())
			c.Writer.Flush()
			return
		}

		return
	}

	var resp dto.ChatResponse
	var err error

	messages := make([]models.Message, len(req.Messages))
	for i, msg := range req.Messages {
		role, _ := msg["role"].(string)
		content := msg["content"]
		messages[i] = models.Message{Role: role, Content: content}
	}

	result, err := h.modelSvr.ChatToModel(c.Request.Context(), providerID, req.ModelName, req.ModelID, messages, modelCfg, chatCfg)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, service.ErrModelNameRequired), errors.Is(err, service.ErrUnsupportedProvider), errors.Is(err, service.ErrProviderIDRequired):
			status = http.StatusBadRequest
		case errors.Is(err, service.ErrProviderNotFound):
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{
			"code":    status,
			"message": err.Error(),
		})
		return
	}
	resp = *result
	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"data":    resp,
		"message": "success",
	})
}

// ListSupportedModels handles fetching models supported by a provider.
func (h *ModelHandler) ListSupportedModels(c *gin.Context) {
	providerID := strings.TrimSpace(c.Param("provider_id"))
	if providerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "provider id is empty",
		})
		return
	}

	result, err := h.modelSvr.ListSupportedModels(c.Request.Context(), providerID, models.ModelConfig{})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrProviderNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{
			"code":    status,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"data":    result,
		"message": "success",
	})
}
