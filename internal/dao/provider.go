package dao

import (
	"FunPDF/internal/common"
	"FunPDF/internal/dto"
	"FunPDF/internal/entity"
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type ProviderDAO struct {
}

func NewProviderDAO() *ProviderDAO {
	return &ProviderDAO{}
}

// GetProviderByID queries a provider by ID.
func (d *ProviderDAO) GetProviderByID(ctx context.Context, db *gorm.DB, providerID string) (*entity.Provider, error) {
	var provider entity.Provider
	err := db.WithContext(ctx).
		Where("id = ?", providerID).
		First(&provider).
		Error
	if err != nil {
		return nil, err
	}
	return &provider, nil
}

// ListProviders list all providers that is already in DB
func (d *ProviderDAO) ListProviders(ctx context.Context, db *gorm.DB) ([]dto.ListProvidersResult, error) {
	var providers []dto.ListProvidersResult
	err := db.WithContext(ctx).Model(&entity.Provider{}).Find(&providers).Error
	if err != nil {
		return nil, err
	}
	return providers, nil
}

// CreateProvider create a provider
func (d *ProviderDAO) CreateProvider(ctx context.Context, db *gorm.DB, req *dto.CreateProviderRequest) (*entity.Provider, error) {
	if strings.TrimSpace(req.URLSuffix["chat"]) == "" || strings.TrimSpace(req.URLSuffix["models"]) == "" {
		return nil, errors.New("provider chat and models url suffix are required")
	}
	id := common.GenerateUUIDv7()
	provider := entity.Provider{
		ID:        id,
		Name:      req.Name,
		BaseURL:   req.BaseURL,
		URLSuffix: req.URLSuffix,
		APIKey:    req.APIKey,
	}
	err := db.WithContext(ctx).Model(&entity.Provider{}).Create(&provider).Error
	if err != nil {
		return nil, err
	}

	provider.APIKey = ""
	return &provider, nil
}

// UpdateProvider update provider
func (d *ProviderDAO) UpdateProvider(ctx context.Context, db *gorm.DB, req *dto.UpdateProviderRequest, providerID string) (int64, error) {
	if strings.TrimSpace(req.URLSuffix["chat"]) == "" || strings.TrimSpace(req.URLSuffix["models"]) == "" {
		return 0, errors.New("provider chat and models url suffix are required")
	}
	update := entity.Provider{
		BaseURL:   req.BaseURL,
		URLSuffix: req.URLSuffix,
		APIKey:    req.APIKey,
	}
	result := db.WithContext(ctx).Model(&entity.Provider{}).
		Where("id = ?", providerID).
		Updates(update)
	return result.RowsAffected, result.Error
}

// DeleteProvider delete provider and it's models
func (d *ProviderDAO) DeleteProvider(ctx context.Context, db *gorm.DB, providerID string) error {
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// delete provider
		err := tx.WithContext(ctx).Model(&entity.Provider{}).
			Where("id = ?", providerID).
			Delete(&entity.Provider{}).
			Error
		if err != nil {
			return fmt.Errorf("delete provider error: %s", err.Error())
		}

		// delete models that related to provider
		var models []entity.ProviderModel
		err = tx.WithContext(ctx).Where("provider_id = ?", providerID).Find(&models).Error
		if err != nil {
			return fmt.Errorf("delete provider error: %s", err.Error())
		}

		for _, model := range models {
			err = tx.WithContext(ctx).Model(&entity.Model{}).
				Where("id = ?", model.ModelID).
				Delete(&entity.Model{}).
				Error
			if err != nil {
				return fmt.Errorf("delete model error: %s", err.Error())
			}
		}

		// delete relationship
		err = tx.WithContext(ctx).Model(&entity.ProviderModel{}).
			Where("provider_id = ?", providerID).
			Delete(&entity.ProviderModel{}).Error
		if err != nil {
			return fmt.Errorf("delete provider-model relationship error: %s", err.Error())
		}

		return nil
	})
	return err
}
