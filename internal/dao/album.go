package dao

import (
	"FunPDF/internal/dto"
	"FunPDF/internal/entity"
	"context"
	"fmt"

	"gorm.io/gorm"
)

type AlbumDAO struct {
}

func NewAlbumDAO() *AlbumDAO {
	return &AlbumDAO{}
}

// ListAlbums list all albums
func (d *AlbumDAO) ListAlbums(ctx context.Context, db *gorm.DB) ([]*entity.Album, error) {
	var albums []*entity.Album
	err := db.WithContext(ctx).Model(&entity.Album{}).Find(&albums).Error
	if err != nil {
		return nil, err
	}
	return albums, nil
}

// CreateAlbum create an album
func (d *AlbumDAO) CreateAlbum(ctx context.Context, db *gorm.DB, album *entity.Album) (int64, error) {
	result := db.WithContext(ctx).Model(&entity.Album{}).
		Create(&album)
	return result.RowsAffected, result.Error
}

// ListAlbumFiles list all files under the album
func (d *AlbumDAO) ListAlbumFiles(ctx context.Context, db *gorm.DB, albumID string) ([]*entity.File, error) {
	var files []*entity.File
	err := db.WithContext(ctx).Table("files").
		Select("files.*").
		Joins("INNER JOIN album_files ON album_files.file_id = files.id").
		Where("album_files.album_id = ?", albumID).
		Distinct().
		Find(&files).Error
	if err != nil {
		return nil, err
	}
	return files, nil
}

// UpdateAlbum update the album
func (d *AlbumDAO) UpdateAlbum(ctx context.Context, db *gorm.DB, req *dto.UpdateAlbumRequest) (int64, error) {
	result := db.WithContext(ctx).Model(&entity.Album{}).
		Where("albums.id = ?", req.ID).
		Updates(entity.Album{
			Name:        req.Name,
			Thumbnail:   req.Thumbnail,
			Description: req.Description,
		})
	return result.RowsAffected, result.Error
}

// DeleteAlbum delete the album(delete album not have influence on normal files)
func (d *AlbumDAO) DeleteAlbum(ctx context.Context, db *gorm.DB, albumID string) error {
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Where("album_id = ?", albumID).
			Delete(&entity.AlbumFile{}).Error
		if err != nil {
			return err
		}

		err = tx.Delete(&entity.Album{}, "id = ?", albumID).Error
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// UploadFilesToAlbum upload a batch of files to album
func (d *AlbumDAO) UploadFilesToAlbum(ctx context.Context, db *gorm.DB, albumID string, fileIDs []string) (map[string]any, int64, error) {
	unSaved := make(map[string]any)

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, fileID := range fileIDs {
			err := tx.Model(&entity.AlbumFile{}).
				Create(&entity.AlbumFile{
					AlbumID: albumID,
					FileID:  fileID,
				}).Error

			if err != nil {
				unSaved[fileID] = err.Error()
			}
		}
		if len(unSaved) == len(fileIDs) {
			return fmt.Errorf("upload files to album failed: no files to upload")
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	return unSaved, int64(len(fileIDs) - len(unSaved)), nil
}

func (d *AlbumDAO) DeleteFilesFromAlbum(ctx context.Context, db *gorm.DB, albumID string, fileIDs []string) (int64, error) {
	result := db.WithContext(ctx).Model(&entity.AlbumFile{}).
		Where("album_id = ?", albumID).
		Where("file_id IN (?)", fileIDs).
		Delete(&entity.AlbumFile{})
	return result.RowsAffected, result.Error
}
