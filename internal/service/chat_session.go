package service

import (
	"FunPDF/internal/common"
	"FunPDF/internal/dao"
	"FunPDF/internal/dto"
	"FunPDF/internal/engine"
	"FunPDF/internal/entity"
	"FunPDF/internal/entity/models"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

type ChatSessionService struct {
	chatSessionDAO *dao.ChatSessionDAO
	fileDAO        *dao.FileDAO
	dialogDAO      *dao.DialogDAO
	modelSrv       *ModelService
}

func NewChatSessionService() *ChatSessionService {
	return &ChatSessionService{
		chatSessionDAO: dao.NewChatSessionDAO(),
		fileDAO:        dao.NewFileDAO(),
		dialogDAO:      dao.NewDialogDAO(),
		modelSrv:       NewModelService(),
	}
}

func (s *ChatSessionService) SetupChatSession(ctx context.Context, providerID string, req *dto.SetupChatSessionRequest) (string, error) {
	fileID := strings.TrimSpace(req.FileID)
	modelID := strings.TrimSpace(req.ModelID)
	modelName := strings.TrimSpace(req.ModelName)
	systemPrompt := strings.TrimSpace(req.SystemPrompt)
	providerID = strings.TrimSpace(providerID)

	if fileID == "" || modelID == "" || modelName == "" || providerID == "" {
		return "", errors.New("invalid request parameter")
	}
	if _, err := s.fileDAO.GetFileByID(ctx, fileID, dao.DB); err != nil {
		return "", err
	}
	// use our system prompt
	if systemPrompt == "" {
		systemPrompt = common.SystemPrompt
	}

	return s.chatSessionDAO.SetupChatSession(ctx, dao.DB, providerID, modelID, modelName, fileID, systemPrompt)
}

func (s *ChatSessionService) DeleteSession(ctx context.Context, providerID, sessionID string) error {
	providerID = strings.TrimSpace(providerID)
	sessionID = strings.TrimSpace(sessionID)
	if providerID == "" || sessionID == "" {
		return errors.New("invalid request parameter")
	}

	return s.chatSessionDAO.DeleteSession(ctx, dao.DB, providerID, sessionID)
}

func (s *ChatSessionService) SendMessages(ctx context.Context, providerID, sessionID string, req *dto.SendMessageRequest, sender func(*string, *string) error) (*dto.ChatResponse, error) {
	providerID = strings.TrimSpace(providerID)
	sessionID = strings.TrimSpace(sessionID)
	content := strings.TrimSpace(req.Content)
	quote := strings.TrimSpace(req.Quote)
	if providerID == "" || sessionID == "" || content == "" {
		return nil, errors.New("invalid request parameter")
	}

	session, err := s.chatSessionDAO.GetChatSession(ctx, dao.DB, sessionID)
	if err != nil {
		return nil, err
	}
	if session.ProviderID != providerID {
		return nil, errors.New("session does not belong to provider")
	}
	dialogList, err := s.dialogDAO.GetTopPDialog(ctx, dao.DB, sessionID, 0)
	if err != nil {
		return nil, err
	}

	fileRecord, err := s.fileDAO.GetFileByID(ctx, session.FileID, dao.DB)
	if err != nil {
		return nil, err
	}

	documentText, ok := engine.PDFText.Get(session.FileID)
	if !ok {
		pdfPath := filepath.Join(CurrentCacheDir(), fileRecord.FileStorageKey, "source.pdf")

		text, err := ExtractPDFText(pdfPath)
		if err != nil {
			return nil, err
		}

		documentText = text
		engine.PDFText.Set(session.FileID, documentText)
	}

	messages := []models.Message{
		{
			Role:    "system",
			Content: session.SystemPrompt,
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("The current PDF: %s\n\nAll content: \n%s", fileRecord.Name, documentText),
		},
	}

	for _, dialog := range dialogList {
		if dialog.Status == 1 {
			messages = append(messages, dialog.Message)
		}
	}
	userContent := content
	if quote != "" {
		userContent = fmt.Sprintf("User's quote: \n%s\n\nUser's question: \n%s", quote, content)
	}

	userMessage := models.Message{
		Role:    "user",
		Content: userContent,
	}
	messages = append(messages, userMessage)

	chatCfg := models.ChatConfig{
		Stream:      req.Stream,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Thinking:    req.Thinking,
		MaxTokens:   req.MaxTokens,
		Effort:      req.Effort,
	}

	userDialog := entity.Dialog{
		ID:        common.GenerateUUID(),
		SessionID: sessionID,
		Message:   userMessage,
		Status:    0,
	}
	assistantDialog := entity.Dialog{
		ID:        common.GenerateUUID(),
		SessionID: sessionID,
		Message:   models.Message{Role: "assistant"},
		Status:    0,
	}

	resp, err := s.modelSrv.ChatToModel(ctx, providerID, session.ModelName, session.ModelID, messages, models.ModelConfig{}, chatCfg, sender)
	if err != nil {
		if saveErr := s.dialogDAO.SaveDialog(ctx, dao.DB, userDialog, assistantDialog); saveErr != nil {
			return nil, saveErr
		}
		return nil, err
	}

	userDialog.Status = 1

	answer := ""
	if resp != nil && resp.Answer != nil {
		answer = *resp.Answer
	}
	assistantDialog.Message = models.Message{Role: "assistant", Content: answer}
	if answer != "" {
		assistantDialog.Status = 1
	}

	if err = s.dialogDAO.SaveDialog(ctx, dao.DB, userDialog, assistantDialog); err != nil {
		return nil, err
	}
	return resp, nil
}
