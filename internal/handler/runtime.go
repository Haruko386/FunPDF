package handler

import (
	"FunPDF/internal/common"
	"FunPDF/internal/dto"
	"FunPDF/internal/service"
	"errors"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

type RuntimeInfo struct {
	Mode         string `json:"mode"`
	Version      string `json:"version"`
	Database     string `json:"database"`
	DatabasePath string `json:"database_path,omitempty"`
	CacheDir     string `json:"cache_dir"`
}

type RuntimeHandler struct {
	info       RuntimeInfo
	runtimeSvr *service.RuntimeService
}

func NewRuntimeHandler(info RuntimeInfo) *RuntimeHandler {
	info.Version = common.GetVersion()
	return &RuntimeHandler{info: info, runtimeSvr: service.NewRuntimeService()}
}

// currentRuntimeInfo returns runtime info with live cache dir and version.
func (h *RuntimeHandler) currentRuntimeInfo() RuntimeInfo {
	info := h.info
	info.Version = common.GetVersion()
	info.CacheDir = service.CurrentCacheDir()
	return info
}

func (h *RuntimeHandler) Info(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"data": h.currentRuntimeInfo(),
		"msg":  "success",
	})
}

func (h *RuntimeHandler) OpenPath(c *gin.Context) {
	var req dto.OpenPathRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  err.Error(),
		})
		return
	}
	path, err := h.runtimeSvr.OpenPath(c.Request.Context(), req)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, fs.ErrNotExist) {
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
		"data": gin.H{

			"path": path,
		},
		"msg": "success",
	})
}

// SelectCacheDir picks a new folder via the native dialog, migrates the file
// storage there and returns the updated runtime info.
func (h *RuntimeHandler) SelectCacheDir(c *gin.Context) {
	if h.info.Mode != "desktop" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "Only support desktop version",
		})
		return
	}

	dir, err := h.runtimeSvr.PickCacheDir()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  err.Error(),
		})
		return
	}

	if dir != "" {
		if _, err := h.runtimeSvr.ChangeCacheDir(c.Request.Context(), dir); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code": http.StatusInternalServerError,
				"msg":  err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"data": h.currentRuntimeInfo(),
		"msg":  "success",
	})
}
