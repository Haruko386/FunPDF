package service

import (
	"FunPDF/internal/common"
	"FunPDF/internal/dao"
	"FunPDF/internal/dto"
	"FunPDF/internal/engine"
	"FunPDF/internal/entity"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type FileService struct {
	fileDAO *dao.FileDAO
}

func NewFileService() *FileService {
	return &FileService{fileDAO: dao.NewFileDAO()}
}

// ListFiles List all files in local
func (s *FileService) ListFiles(ctx context.Context) ([]entity.File, error) {
	fileList, err := s.fileDAO.ListFiles(ctx, dao.DB)
	return fileList, err
}

// GetFile get file content
func (s *FileService) GetFile(ctx context.Context, fileID string) (string, error) {
	fileRecord, err := s.fileDAO.GetFileByID(ctx, fileID, dao.DB)
	if err != nil {
		return "", err
	}

	filePath := filepath.Join(CurrentCacheDir(), fileRecord.FileStorageKey, "source.pdf")
	go func(fileID, path string) {
		if _, ok := engine.PDFText.Get(fileID); ok {
			return
		}

		text, err := ExtractPDFText(path)
		if err != nil {
			common.Warn(fmt.Sprintf("extract text failed: %v", err))
			return
		}
		engine.PDFText.Set(fileID, text)
	}(fileID, filePath)

	return filePath, nil
}

// GetFileState get file state
func (s *FileService) GetFileState(ctx context.Context, fileID string) (json.RawMessage, error) {
	fileRecord, err := s.fileDAO.GetFileByID(ctx, fileID, dao.DB)
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(CurrentCacheDir(), fileRecord.FileStorageKey, "editor-state.json")
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	jsonBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return jsonBytes, nil
}

// GetFileThumbnail get file thumbnail
func (s *FileService) GetFileThumbnail(ctx context.Context, fileID string) (string, error) {
	thumbnail, err := s.fileDAO.GetFileThumbnail(ctx, dao.DB, fileID)
	if err != nil || thumbnail == "" {
		return "", err
	}
	return thumbnail, nil
}

// SaveFile file to local storage
func (s *FileService) SaveFile(ctx context.Context, fileID string, req *dto.SaveFileRequest) (bool, error) {
	file, err := s.fileDAO.GetFileByID(ctx, fileID, dao.DB)
	if err != nil {
		return false, err
	}

	if req.ExpectedRevision != file.Revision {
		return false, fmt.Errorf("revision mismatch: expected %d, got %d", file.Revision, req.ExpectedRevision)
	}

	projectDir := strings.TrimSpace(file.FileStorageKey)
	if projectDir == "" {
		return false, fmt.Errorf("file location is empty")
	}
	stateDir := filepath.Join(CurrentCacheDir(), projectDir)
	statePath := filepath.Join(stateDir, "editor-state.json")
	bakPath := statePath + ".bak"

	stateData, err := os.ReadFile(statePath)
	if err != nil {
		return false, fmt.Errorf("read state file failed: %v", err)
	}

	var stateJSON map[string]any
	if err := json.Unmarshal(stateData, &stateJSON); err != nil {
		return false, fmt.Errorf("unmarshal state file failed: %v", err)
	}
	var editorState map[string]any
	if err := json.Unmarshal(req.EditorState, &editorState); err != nil {
		return false, fmt.Errorf("unmarshal editor state failed: %v", err)
	}
	for key, value := range editorState {
		stateJSON[key] = value
	}

	updatedData, err := json.Marshal(stateJSON)
	if err != nil {
		return false, fmt.Errorf("marshal state file failed: %v", err)
	}

	tmp, err := os.CreateTemp(stateDir, ".editor-state-*.tmp")
	if err != nil {
		return false, fmt.Errorf("create temp file failed: %v", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err = tmp.Write(updatedData); err != nil {
		tmp.Close()
		return false, fmt.Errorf("write temp file failed: %v", err)
	}
	if err = tmp.Close(); err != nil {
		return false, fmt.Errorf("close temp file failed: %v", err)
	}

	_ = os.Remove(bakPath)
	if err := os.Rename(statePath, bakPath); err != nil {
		return false, fmt.Errorf("backup original failed: %v", err)
	}

	if err := os.Rename(tmpName, statePath); err != nil {
		_ = os.Rename(bakPath, statePath)
		return false, fmt.Errorf("replace state file failed: %v", err)
	}

	var affected int64
	if req.Thumbnail != nil {
		affected, err = s.fileDAO.SaveThumbnail(ctx, fileID, dao.DB, req)
	} else {
		affected, err = s.fileDAO.AdvanceRevision(ctx, fileID, dao.DB, file.Revision, file.Revision+1)
	}
	if err != nil {
		_ = os.Remove(statePath)
		_ = os.Rename(bakPath, statePath)
		return false, fmt.Errorf("update revision failed: %v", err)
	}
	if affected != 1 {
		_ = os.Remove(statePath)
		_ = os.Rename(bakPath, statePath)
		return false, fmt.Errorf("revision conflict")
	}

	_ = os.Remove(bakPath)
	return true, nil
}

// UploadFile upload the file to local for first time save
func (s *FileService) UploadFile(ctx context.Context, req *dto.UploadFileRequest, source io.Reader) (_ *entity.File, resultErr error) {
	fileID := common.GenerateUUIDv7()

	if err := os.MkdirAll(CurrentCacheDir(), 0700); err != nil {
		return nil, err
	}

	tempDir, err := os.MkdirTemp(CurrentCacheDir(), "."+fileID+"-")
	if err != nil {
		return nil, err
	}

	defer func() {
		if resultErr != nil {
			_ = os.RemoveAll(tempDir)
		}
	}()

	pdfPath := filepath.Join(tempDir, "source.pdf")
	pdfFile, err := os.OpenFile(pdfPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}

	hash := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(pdfFile, hash), source)
	syncErr := pdfFile.Sync()
	closeErr := pdfFile.Close()

	if copyErr != nil {
		return nil, copyErr
	}
	if syncErr != nil {
		return nil, syncErr
	}
	if closeErr != nil {
		return nil, closeErr
	}

	sha := hex.EncodeToString(hash.Sum(nil))

	statePath := filepath.Join(tempDir, "editor-state.json")
	if err := saveJSON(statePath, req.EditorState); err != nil {
		return nil, err
	}

	finalDir := filepath.Join(CurrentCacheDir(), fileID)
	if err := os.Rename(tempDir, finalDir); err != nil {
		return nil, err
	}
	tempDir = finalDir

	file := &entity.File{
		ID:             fileID,
		Name:           filepath.Base(req.FileName),
		MimeType:       "application/pdf",
		FileStorageKey: fileID,
		Size:           size,
		SHA256:         sha,
		Revision:       1,
		Status:         "ready",
	}

	affected, err := s.fileDAO.UploadFile(ctx, file, dao.DB)
	if err != nil || affected != 1 {
		return nil, fmt.Errorf("create file record failed: %v", err)
	}

	tempDir = ""
	return file, nil
}

// AlertFile update file's metadata
func (s *FileService) AlertFile(ctx context.Context, fileID string, req *dto.AlertFileRequest) (*entity.File, error) {
	fileName := strings.TrimSpace(req.Name)
	if fileName == "" {
		return nil, fmt.Errorf("file name is empty")
	}

	mimeType := strings.TrimSpace(req.MimeType)
	if mimeType == "" {
		return nil, fmt.Errorf("mime_type is empty")
	}

	req.Name = fileName
	req.MimeType = mimeType

	file, err := s.fileDAO.AlertFile(ctx, dao.DB, fileID, req)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// DeleteFile delete the file
func (s *FileService) DeleteFile(ctx context.Context, fileID string) (int64, error) {
	file, err := s.fileDAO.GetFileByID(ctx, fileID, dao.DB)
	if err != nil {
		return 0, fmt.Errorf("get file failed: %v", err)
	}

	projectDir := strings.TrimSpace(file.FileStorageKey)
	if projectDir == "" {
		return 0, fmt.Errorf("file location is empty")
	}
	srcDir := filepath.Join(CurrentCacheDir(), projectDir)

	trashDir := filepath.Join(CurrentCacheDir(), ".trash")
	trashPath := filepath.Join(trashDir, fileID)

	if err := os.MkdirAll(trashDir, 0700); err != nil {
		return 0, fmt.Errorf("create trash dir failed: %v", err)
	}
	if err := os.Rename(srcDir, trashPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return 0, fmt.Errorf("move to trash failed: %v", err)
		}
	}

	affected, err := s.fileDAO.DeleteFile(ctx, fileID, dao.DB)
	if err != nil || affected == 0 {
		_ = os.Rename(trashPath, srcDir)
		return 0, fmt.Errorf("delete file record failed: %v", err)
	}

	_ = os.RemoveAll(trashPath)
	engine.PDFText.Delete(fileID)
	return affected, nil
}

// ListFileAlbums get file's album
func (s *FileService) ListFileAlbums(ctx context.Context, fileID string) ([]entity.Album, error) {
	albums, err := s.fileDAO.ListFileAlbums(ctx, dao.DB, fileID)
	if err != nil {
		return nil, err
	}
	return albums, nil
}

func (s *FileService) DeleteFileCache(ctx context.Context, fileID string) {
	engine.PDFText.Delete(strings.TrimSpace(fileID))
}
