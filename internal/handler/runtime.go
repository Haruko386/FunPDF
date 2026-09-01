package handler

import (
	"FunPDF/internal/common"
	"FunPDF/internal/dto"
	"FunPDF/internal/service"
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

func (h *RuntimeHandler) Info(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"data": h.info,
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
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
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
