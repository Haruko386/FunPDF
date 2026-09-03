package handler

import (
	"FunPDF/internal/dto"
	"FunPDF/internal/service"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type ProviderHandler struct {
	providerSvr *service.ProviderService
}

func NewProviderHandler() *ProviderHandler {
	return &ProviderHandler{
		providerSvr: service.NewProviderService(),
	}
}

// ListProviders list supported providers
func (h *ProviderHandler) ListProviders(c *gin.Context) {
	result, err := h.providerSvr.ListProviders(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"data": result,
		"msg":  "success",
	})
}

// CreateProvider Create a custom provider
func (h *ProviderHandler) CreateProvider(c *gin.Context) {
	var req dto.CreateProviderRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  err.Error(),
		})
		return
	}

	provider, err := h.providerSvr.CreateProvider(c.Request.Context(), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrProviderNameRequired) || errors.Is(err, service.ErrProviderURLSuffix) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{
			"code": status,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code": http.StatusCreated,
		"data": provider,
		"msg":  "success",
	})
}

func (h *ProviderHandler) UpdateProvider(c *gin.Context) {
	var req dto.UpdateProviderRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  err.Error(),
		})
		return
	}

	providerID := strings.TrimSpace(c.Param("provider_id"))

	err := h.providerSvr.UpdateProvider(c.Request.Context(), &req, providerID)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, service.ErrProviderIDRequired), errors.Is(err, service.ErrProviderURLSuffix):
			status = http.StatusBadRequest
		case errors.Is(err, service.ErrProviderNotFound):
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
		"msg":  "success",
	})
}

func (h *ProviderHandler) DeleteProvider(c *gin.Context) {
	providerID := strings.TrimSpace(c.Param("provider_id"))
	if providerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "provider id is required",
		})
		return
	}

	err := h.providerSvr.DeleteProvider(c.Request.Context(), providerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  err.Error(),
		})
		return
	}

	c.Status(http.StatusNoContent)
}
