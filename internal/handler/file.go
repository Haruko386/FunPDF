package handler

import (
	"FunPDF/internal/dto"
	"FunPDF/internal/service"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type FileHandler struct {
	fileSvr *service.FileService
}

func NewFileHandler() *FileHandler {
	return NewFileHandlerWithService(service.NewFileService())
}

func NewFileHandlerWithService(fileSvr *service.FileService) *FileHandler {
	if fileSvr == nil {
		return &FileHandler{fileSvr: service.NewFileService()}
	}
	return &FileHandler{fileSvr: fileSvr}
}

// ListFiles list all files
func (h *FileHandler) ListFiles(c *gin.Context) {
	fileList, err := h.fileSvr.ListFiles(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"data": fileList,
		"msg":  "success",
	})
}

// GetFile get file content
func (h *FileHandler) GetFile(c *gin.Context) {
	fileID := strings.TrimSpace(c.Param("file_id"))
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "file id is empty",
		})
		return
	}

	filePath, err := h.fileSvr.GetFile(c.Request.Context(), fileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  err.Error(),
		})
		return
	}

	c.Header("Content-Type", "application/pdf")
	c.File(filePath)
}

// GetFileState get file state
func (h *FileHandler) GetFileState(c *gin.Context) {
	fileID := strings.TrimSpace(c.Param("file_id"))
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "file id is empty",
		})
		return
	}

	jsonFile, err := h.fileSvr.GetFileState(c.Request.Context(), fileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"data": jsonFile,
		"msg":  "success",
	})
}

// GetFileThumbnail get file thumbnail
func (h *FileHandler) GetFileThumbnail(c *gin.Context) {
	fileID := strings.TrimSpace(c.Param("file_id"))
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "file id is empty",
		})
		return
	}

	thumbnail, err := h.fileSvr.GetFileThumbnail(c.Request.Context(), fileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"data": thumbnail,
		"msg":  "success",
	})
}

// SaveFile saves the edited JSON state to local storage and thumbnail to DB.
func (h *FileHandler) SaveFile(c *gin.Context) {
	fileID := strings.TrimSpace(c.Param("file_id"))
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "file id is empty",
		})
		return
	}

	var req dto.SaveFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  err.Error(),
		})
		return
	}

	ok, err := h.fileSvr.SaveFile(c.Request.Context(), fileID, &req)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "revision") {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{
			"code": status,
			"msg":  err.Error(),
		})
		return
	}
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  "save failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"msg":  "success",
	})
}

// UploadFile stores the PDF and initial editor state on first Ctrl+S.
func (h *FileHandler) UploadFile(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 200<<20)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  err.Error(),
		})
		return
	}

	stateText := strings.TrimSpace(c.PostForm("editor_state"))
	if stateText == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "editor_state is empty",
		})
		return
	}
	if !json.Valid([]byte(stateText)) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "editor_state is invalid JSON",
		})
		return
	}

	source, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  err.Error(),
		})
		return
	}
	defer source.Close()

	req := &dto.UploadFileRequest{
		FileName:    fileHeader.Filename,
		MimeType:    fileHeader.Header.Get("Content-Type"),
		FileSize:    fileHeader.Size,
		EditorState: json.RawMessage(stateText),
	}
	file, err := h.fileSvr.UploadFile(c.Request.Context(), req, source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code": http.StatusCreated,
		"msg":  "success",
		"data": file,
	})
}

// AlertFile update file's metadata
func (h *FileHandler) AlertFile(c *gin.Context) {
	fileID := strings.TrimSpace(c.Param("file_id"))
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "file id is empty",
		})
		return
	}

	var req dto.AlertFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  err.Error(),
		})
		return
	}

	file, err := h.fileSvr.AlertFile(c.Request.Context(), fileID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"data": file,
		"msg":  "success",
	})
}

// DeleteFile delete the file
func (h *FileHandler) DeleteFile(c *gin.Context) {
	fileID := strings.TrimSpace(c.Param("file_id"))
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "file id is empty",
		})
		return
	}
	affected, err := h.fileSvr.DeleteFile(c.Request.Context(), fileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  err.Error(),
		})
		return
	}
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"code": http.StatusNotFound,
			"msg":  "file id is not exist",
		})
		return
	}
	c.Status(http.StatusNoContent)
}

// ListFileAlbums get file's album
func (h *FileHandler) ListFileAlbums(c *gin.Context) {
	fileID := strings.TrimSpace(c.Param("file_id"))
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "file id is empty",
		})
		return
	}

	albums, err := h.fileSvr.ListFileAlbums(c.Request.Context(), fileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"data": albums,
		"msg":  "success",
	})
}

func (h *FileHandler) DeleteFileCache(c *gin.Context) {
	fileID := strings.TrimSpace(c.Param("file_id"))
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "file id is empty",
		})
		return
	}

	h.fileSvr.DeleteFileCache(c.Request.Context(), fileID)

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"msg":  "success",
	})
}
