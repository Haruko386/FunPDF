package service

import (
	"FunPDF/internal/common"
	"FunPDF/internal/dao"
	"FunPDF/internal/dto"
	"FunPDF/internal/entity"
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type AlbumService struct {
	albumDAO *dao.AlbumDAO
}

func NewAlbumService() *AlbumService {
	return &AlbumService{albumDAO: dao.NewAlbumDAO()}
}

// ListAlbums list all albums
func (s *AlbumService) ListAlbums(ctx context.Context) ([]*entity.Album, error) {
	albums, err := s.albumDAO.ListAlbums(ctx, dao.DB)
	if err != nil {
		return nil, err
	}
	return albums, nil
}

// CreateAlbum create an album
func (s *AlbumService) CreateAlbum(ctx context.Context, req *dto.CreateAlbumRequest) (*entity.Album, error) {
	albumID := common.GenerateUUIDv7()
	albumName := strings.TrimSpace(req.Name)
	if albumName == "" {
		return nil, ErrAlbumNameRequired
	}

	// check thumbnail size
	if err := ValidateBase64ImageSize(req.Thumbnail); err != nil {
		return nil, err
	}

	album := &entity.Album{
		ID:          albumID,
		Name:        albumName,
		Thumbnail:   req.Thumbnail,
		Description: req.Description,
	}

	affected, err := s.albumDAO.CreateAlbum(ctx, dao.DB, album)
	if err != nil || affected == 0 {
		return nil, fmt.Errorf("create album failed: %w", err)
	}
	return album, nil
}

// ListAlbumFiles list all files under the album
func (s *AlbumService) ListAlbumFiles(ctx context.Context, albumID string) ([]*entity.File, error) {
	result, err := s.albumDAO.ListAlbumFiles(ctx, dao.DB, albumID)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// UpdateAlbum update the album
func (s *AlbumService) UpdateAlbum(ctx context.Context, req *dto.UpdateAlbumRequest) error {
	if err := ValidateBase64ImageSize(req.Thumbnail); err != nil {
		return fmt.Errorf("invalid thumbnail size: %w", err)
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)

	affected, err := s.albumDAO.UpdateAlbum(ctx, dao.DB, req)
	if err != nil {
		return fmt.Errorf("update album failed: %w", err)
	}
	if affected == 0 {
		return ErrAlbumNotFound
	}

	return nil
}

// DeleteAlbum delete the album
func (s *AlbumService) DeleteAlbum(ctx context.Context, albumID string) error {
	err := s.albumDAO.DeleteAlbum(ctx, dao.DB, albumID)
	if err != nil {
		return fmt.Errorf("delete album failed: %w", err)
	}
	return nil
}

// UploadFilesToAlbum upload a batch of files to album
func (s *AlbumService) UploadFilesToAlbum(ctx context.Context, albumID string, req *dto.AlertAlbumFilesRequest) (map[string]any, error) {
	validIDs := checkDuplicateIDs(req.IDs)
	if len(validIDs) == 0 {
		return nil, nil
	}

	unSaved, affected, err := s.albumDAO.UploadFilesToAlbum(ctx, dao.DB, albumID, validIDs)
	if err != nil {
		if errors.Is(err, gorm.ErrInvalidData) {
			return nil, nil
		}
		return nil, fmt.Errorf("upload files to album failed: %w", err)
	}

	if affected == 0 {
		return nil, fmt.Errorf("upload files to album failed: no files to upload")
	}

	return unSaved, nil
}

// DeleteFilesFromAlbum delete a batch of files from album
func (s *AlbumService) DeleteFilesFromAlbum(ctx context.Context, albumID string, req *dto.AlertAlbumFilesRequest) error {
	validIDs := checkDuplicateIDs(req.IDs)
	if len(validIDs) == 0 {
		return nil
	}

	affected, err := s.albumDAO.DeleteFilesFromAlbum(ctx, dao.DB, albumID, validIDs)
	if err != nil || affected == 0 {
		return fmt.Errorf("delete files from album failed: %w", err)
	}
	if int(affected) != len(validIDs) {
		return fmt.Errorf("delete files from album failed: invalid number of files to delete")
	}

	return nil
}
