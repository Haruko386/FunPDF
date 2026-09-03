package dao

import (
	"FunPDF/internal/common"
	"FunPDF/internal/entity"
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

type ChatSessionDAO struct{}

func NewChatSessionDAO() *ChatSessionDAO {
	return &ChatSessionDAO{}
}

// SetupChatSession create a chat session log
func (d *ChatSessionDAO) SetupChatSession(ctx context.Context, db *gorm.DB, providerID, modelID, modelName, fileID, systemPrompt string) (sessionID string, err error) {
	sessionID = common.GenerateUUID()
	err = db.WithContext(ctx).Model(&entity.ChatSession{}).
		Create(&entity.ChatSession{
			ID:           sessionID,
			FileID:       fileID,
			ProviderID:   providerID,
			ModelID:      modelID,
			ModelName:    modelName,
			SystemPrompt: systemPrompt,
		}).Error
	if err != nil {
		return "", err
	}
	return sessionID, nil
}

func (d *ChatSessionDAO) DeleteSession(ctx context.Context, db *gorm.DB, providerID, sessionID string) error {
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("id = ? AND provider_id = ?", sessionID, providerID).Delete(&entity.ChatSession{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("session %s not found: %w", sessionID, gorm.ErrRecordNotFound)
		}
		if err := tx.Where("session_id = ?", sessionID).Delete(&entity.Dialog{}).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// GetChatSession get chat session by ID
func (d *ChatSessionDAO) GetChatSession(ctx context.Context, db *gorm.DB, sessionID string) (*entity.ChatSession, error) {
	var chatSession entity.ChatSession
	err := db.WithContext(ctx).Model(&entity.ChatSession{}).
		Where("id = ?", sessionID).
		First(&chatSession).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("Session %s not found: %w", sessionID, gorm.ErrRecordNotFound)
		}
		return nil, err
	}
	return &chatSession, nil
}

func (d *ChatSessionDAO) GetDialogStatus(ctx context.Context, db *gorm.DB, dialogID string) (status int, err error) {
	err = db.WithContext(ctx).Model(&entity.Dialog{}).
		Select("status").
		First(&status, "id = ?", dialogID).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, fmt.Errorf("dialog not found")
		}
		return status, err
	}
	return status, nil
}

func (d *ChatSessionDAO) UpdateDialogStatus(ctx context.Context, db *gorm.DB, dialogID string, status int) (err error) {
	err = db.WithContext(ctx).Model(&entity.Dialog{}).
		Where("id = ?", dialogID).
		Updates(map[string]any{
			"status": status,
		}).Error
	return err
}
