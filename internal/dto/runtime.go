package dto

type OpenPathRequest struct {
	Path string `json:"path" binding:"required"`
}
