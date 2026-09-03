package handler

import (
	"FunPDF/internal/dto"
	"FunPDF/internal/service"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ChatSessionHandler struct {
	chatSessionSvr *service.ChatSessionService
}

func NewChatSessionHandler() *ChatSessionHandler {
	return NewChatSessionHandlerWithService(service.NewChatSessionService())
}

func NewChatSessionHandlerWithService(chatSessionSvr *service.ChatSessionService) *ChatSessionHandler {
	return &ChatSessionHandler{
		chatSessionSvr: chatSessionSvr,
	}
}

// SetupChatSession set up session when first time chat
func (h *ChatSessionHandler) SetupChatSession(c *gin.Context) {
	providerID := strings.TrimSpace(c.Param("provider_id"))
	if providerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "provider id is empty",
		})
		return
	}

	var req dto.SetupChatSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  err.Error(),
		})
		return
	}

	session, err := h.chatSessionSvr.SetupChatSession(c.Request.Context(), providerID, &req)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{
			"code": status,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"data": gin.H{"id": session},
		"msg":  "success",
	})
}

func (h *ChatSessionHandler) DeleteSession(c *gin.Context) {
	providerID := strings.TrimSpace(c.Param("provider_id"))
	if providerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "provider id is empty",
		})
		return
	}

	sessionID := strings.TrimSpace(c.Param("session_id"))
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "session id is empty",
		})
		return
	}

	if err := h.chatSessionSvr.DeleteSession(c.Request.Context(), providerID, sessionID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{
			"code": status,
			"msg":  err.Error(),
		})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *ChatSessionHandler) SendMessages(c *gin.Context) {
	providerID := strings.TrimSpace(c.Param("provider_id"))
	if providerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "provider id is empty",
		})
		return
	}

	sessionID := strings.TrimSpace(c.Param("session_id"))
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "session id is empty",
		})
		return
	}

	var req dto.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  err.Error(),
		})
		return
	}

	if req.Stream != nil && *req.Stream {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Writer.WriteHeader(http.StatusOK)
		c.Writer.Flush()

		sender := func(content, reasoningContent *string) error {
			if content != nil {
				c.SSEvent("message", gin.H{
					"content":           *content,
					"reasoning_content": "",
				})
				c.Writer.Flush()
			}
			if reasoningContent != nil {
				c.SSEvent("message", gin.H{
					"content":           "",
					"reasoning_content": *reasoningContent,
				})
				c.Writer.Flush()
			}
			return nil
		}

		if _, err := h.chatSessionSvr.SendMessages(c.Request.Context(), providerID, sessionID, &req, sender); err != nil {
			c.SSEvent("error", err.Error())
			c.Writer.Flush()
			return
		}
		c.SSEvent("done", "[DONE]")
		c.Writer.Flush()
		return
	}

	resp, err := h.chatSessionSvr.SendMessages(c.Request.Context(), providerID, sessionID, &req, nil)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{
			"code": status,
			"msg":  err.Error(),
		})
		return
	}

	answer := ""
	reasoning := ""
	if resp != nil && resp.Answer != nil {
		answer = *resp.Answer
	}
	if resp != nil && resp.ReasonContent != nil {
		reasoning = *resp.ReasonContent
	}
	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"data": gin.H{
			"content":           answer,
			"reasoning_content": reasoning,
		},
		"msg": "success",
	})
}
