package handler

import (
	"FunPDF/internal/dto"
	"FunPDF/internal/service"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type TranslatorHandler struct {
	transSvr service.TranslatorService
}

func NewTranslatorHandler() *TranslatorHandler {
	return &TranslatorHandler{
		transSvr: service.NewTranslatorService(),
	}
}

// ListTranslators list all translators
func (h *TranslatorHandler) ListTranslators(c *gin.Context) {
	list, err := h.transSvr.ListTranslators(c.Request.Context())
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

// CreateTranslator create a unique translators
func (h *TranslatorHandler) CreateTranslator(c *gin.Context) {
	var req dto.CreateTranslatorsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": err.Error(),
		})
		return
	}

	translator, err := h.transSvr.CreateTranslator(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    http.StatusCreated,
		"data":    translator,
		"message": "success",
	})
}

// Translate source to dst language
func (h *TranslatorHandler) Translate(c *gin.Context) {
	translatorName := strings.TrimSpace(c.Param("translator_name"))
	if translatorName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "translators name is required",
		})
		return
	}

	var req *dto.TranslateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": err.Error(),
		})
		return
	}

	dst, err := h.transSvr.Translate(c.Request.Context(), req, translatorName)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrTranslatorNotFound) {
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
		"data":    dst,
		"message": "success",
	})
}
