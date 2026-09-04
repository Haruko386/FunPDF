package dao

import (
	"FunPDF/internal/entity"
	"context"

	"gorm.io/gorm"
)

type DialogDAO struct{}

func NewDialogDAO() *DialogDAO {
	return &DialogDAO{}
}

// GetTopPDialog returns the latest dialogs for a chat session.
func (d *DialogDAO) GetTopPDialog(ctx context.Context, db *gorm.DB, sessionID string, topP int) ([]entity.Dialog, error) {
	dialogs := make([]entity.Dialog, 0)

	query := db.WithContext(ctx).Model(&entity.Dialog{}).
		Where("session_id = ?", sessionID).
		Order("created_at ASC")
	if topP > 0 {
		query = query.Limit(topP)
	}

	err := query.Find(&dialogs).Error
	if err != nil {
		return nil, err
	}
	return dialogs, nil
}

// SaveDialog inserts user and assistant dialog records in a transaction.
func (d *DialogDAO) SaveDialog(ctx context.Context, db *gorm.DB, userDialog, assistantDialog entity.Dialog) error {
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&entity.Dialog{}).
			Create(&userDialog).Error; err != nil {
			return err
		}
		if err := tx.Model(&entity.Dialog{}).
			Create(&assistantDialog).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}
