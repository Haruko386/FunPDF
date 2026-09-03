package dao

import (
	"FunPDF/internal/common"
	"FunPDF/internal/entity"
	"context"
	"encoding/json"
	"fmt"
	"log"

	"gorm.io/gorm"
)

type TranslatorDAO struct{}

func NewTranslatorDAO() *TranslatorDAO {
	return &TranslatorDAO{}
}

// GetTranslatorParams get translators's params by translators name
func (d *TranslatorDAO) GetTranslatorParams(ctx context.Context, db *gorm.DB, name string) (json.RawMessage, error) {
	var translator *entity.Translator
	if err := db.WithContext(ctx).First(&translator, "name = ?", name).Error; err != nil {
		return nil, err
	}
	params := translator.Params
	if len(params) == 0 {
		return nil, fmt.Errorf("translators's params has no value, please rebuild the translators: %s", name)
	}
	return params, nil
}

// ListTranslators list all translators
func (d *TranslatorDAO) ListTranslators(ctx context.Context, db *gorm.DB) ([]*entity.Translator, error) {
	var translators []*entity.Translator
	err := db.WithContext(ctx).Find(&translators).Error
	if err != nil {
		return nil, err
	}
	return translators, nil
}

// CreateTranslator create a unique translators
func (d *TranslatorDAO) CreateTranslator(ctx context.Context, db *gorm.DB, translatorName string, params json.RawMessage) (*entity.Translator, error) {
	var existing entity.Translator
	err := db.WithContext(ctx).Where("name = ?", translatorName).First(&existing).Error
	if err == nil {
		log.Printf("translator %s already exists", translatorName)
		return &existing, nil
	}

	var translator entity.Translator
	translator.ID = common.GenerateUUIDv7()
	translator.Name = translatorName
	translator.Params = params

	err = db.WithContext(ctx).Create(&translator).Error
	if err != nil {
		return nil, err
	}
	return &translator, nil
}
