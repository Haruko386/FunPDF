package dao

import (
	"FunPDF/internal/common"
	"FunPDF/internal/dto"
	"FunPDF/internal/entity"
	"context"
	"errors"

	"gorm.io/gorm"
)

type ModelDAO struct{}

func NewModelDAO() *ModelDAO {
	return &ModelDAO{}
}

// ListProviderModel list provider's model stored in DB
func (d *ModelDAO) ListProviderModel(ctx context.Context, db *gorm.DB, providerID string) (*[]dto.ListProviderModelsResponse, error) {
	var list []dto.ListProviderModelsResponse
	err := db.WithContext(ctx).Model(&entity.ProviderModel{}).
		Select("models.id, models.name").
		Joins("inner join models on models.id = provider_models.model_id").
		Where("provider_models.provider_id = ?", providerID).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return &list, nil
}

// SaveProviderModels save some models to DB
func (d *ModelDAO) SaveProviderModels(ctx context.Context, db *gorm.DB, providerID string, modelNames []string) (*[]entity.Model, error) {
	modelList := make([]entity.Model, 0, len(modelNames))
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, modelName := range modelNames {
			id := common.GenerateUUIDv7()
			model := entity.Model{
				ID:   id,
				Name: modelName,
			}
			// store model
			if err := tx.WithContext(ctx).Model(&entity.Model{}).
				Create(&model).Error; err != nil {
				return err
			}

			// store relationship
			if err := tx.WithContext(ctx).Model(&entity.ProviderModel{}).
				Create(&entity.ProviderModel{
					ModelID:    id,
					ProviderID: providerID,
				}).Error; err != nil {
				return err
			}

			modelList = append(modelList, model)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &modelList, nil
}

// DeleteProviderModels deletes selected provider models.
func (d *ModelDAO) DeleteProviderModels(ctx context.Context, db *gorm.DB, providerID string, modelIDs []string) error {
	var affected int64
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, modelID := range modelIDs {
			result := tx.WithContext(ctx).Model(&entity.ProviderModel{}).
				Where("provider_id = ? AND model_id = ?", providerID, modelID).
				Delete(&entity.ProviderModel{})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				continue
			}

			err := tx.WithContext(ctx).Model(&entity.Model{}).
				Where("id = ?", modelID).
				Delete(&entity.Model{}).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}

			affected += 1
		}
		return nil
	})
	if err != nil {
		return err
	}
	if affected != int64(len(modelIDs)) {
		common.Warn("There is some problems in the model model list, some models already been deleted")
	}
	return nil
}
