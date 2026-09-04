package dto

import (
	"encoding/json"
)

type SaveFileRequest struct {
	ExpectedRevision int64           `json:"expected_revision" binding:"required"`
	Thumbnail        *string         `json:"thumbnail"`
	EditorState      json.RawMessage `json:"editor_state" binding:"required"`
}

type UpdateFileRequest struct {
	Name             string `json:"name"`
	MimeType         string `json:"mime_type"`
	Revision         int64  `json:"revision"`
	ExpectedRevision int64  `json:"expected_revision"`
	Status           string `json:"status"`
}

type UploadFileRequest struct {
	FileName    string
	MimeType    string
	FileSize    int64
	EditorState json.RawMessage
}

type AlertFileRequest struct {
	Name     string `json:"name"`
	MimeType string `json:"mime_type"`
}

type ImportLocalFileRequest struct {
	Path string `json:"path" binding:"required"`
}
