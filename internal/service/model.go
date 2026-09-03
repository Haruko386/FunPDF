package service

import (
	"FunPDF/internal/dao"
	"FunPDF/internal/dto"
	"FunPDF/internal/entity"
	"FunPDF/internal/entity/models"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
)

type ModelService struct {
	modelDAO    *dao.ModelDAO
	providerDAO *dao.ProviderDAO
	httpClient  *http.Client
}

func NewModelService() *ModelService {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.ResponseHeaderTimeout = 30 * time.Second

	return &ModelService{
		modelDAO:    dao.NewModelDAO(),
		providerDAO: dao.NewProviderDAO(),
		httpClient: &http.Client{
			Transport: transport,
		},
	}
}

// ListProviderModel list provider's model stored in DB
func (s *ModelService) ListProviderModel(ctx context.Context, providerID string) (*[]dto.ListProviderModelsResponse, error) {
	list, err := s.modelDAO.ListProviderModel(ctx, dao.DB, providerID)
	if err != nil {
		return nil, err
	}
	return list, nil
}

// SaveProviderModels save some models to DB
func (s *ModelService) SaveProviderModels(ctx context.Context, providerID string, req *dto.SaveModelsRequest) (*[]entity.Model, error) {
	if req.Names == nil {
		return nil, errors.New("names is empty")
	}
	savedModelNames := make([]string, 0, len(*req.Names))
	for _, name := range *req.Names {
		name = strings.TrimSpace(name)
		if name != "" {
			savedModelNames = append(savedModelNames, name)
		}
	}

	if len(savedModelNames) == 0 {
		return nil, errors.New("no names provided")
	}

	modelList, err := s.modelDAO.SaveProviderModels(ctx, dao.DB, providerID, savedModelNames)
	if err != nil {
		return nil, err
	}
	return modelList, nil
}

// DeleteProviderModels validates and deletes provider model selections.
func (s *ModelService) DeleteProviderModels(ctx context.Context, providerID string, req *dto.DeleteModelsRequest) error {
	if req.IDs == nil {
		return errors.New("ids is empty")
	}
	modelIDs := make([]string, 0, len(*req.IDs))
	for _, id := range *req.IDs {
		id = strings.TrimSpace(id)
		if id != "" {
			modelIDs = append(modelIDs, id)
		}
	}

	if len(modelIDs) == 0 {
		return errors.New("no ids provided")
	}

	err := s.modelDAO.DeleteProviderModels(ctx, dao.DB, providerID, modelIDs)
	if err != nil {
		return err
	}
	return nil
}

// ChatToModelStreamWithSender sends a streaming chat request through the model service.
func (s *ModelService) ChatToModelStreamWithSender(ctx context.Context, providerID, modelName, modelID string, messages []models.Message, modelCfg models.ModelConfig, chatCfg models.ChatConfig, sender func(*string, *string) error) error {
	resp, err := s.ChatToModel(ctx, providerID, modelName, modelID, messages, modelCfg, chatCfg, sender)
	if err != nil {
		return err
	}
	if resp == nil {
		return nil
	}
	done := "[DONE]"
	return sender(&done, nil)
}

// ChatToModel validates provider/model configuration and sends a chat request.
func (s *ModelService) ChatToModel(ctx context.Context, providerID, modelName, modelID string, messages []models.Message, modelCfg models.ModelConfig, chatCfg models.ChatConfig, sender ...func(*string, *string) error) (*dto.ChatResponse, error) {
	providerID = strings.TrimSpace(providerID)
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		modelName = strings.TrimSpace(modelID)
	}
	if providerID == "" {
		return nil, ErrProviderIDRequired
	}
	if modelName == "" {
		return nil, ErrModelNameRequired
	}

	provider, err := s.providerDAO.GetProviderByID(ctx, dao.DB, providerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProviderNotFound
		}
		return nil, err
	}

	apiKey := strings.TrimSpace(provider.APIKey)
	modelCfg.APIKey = &apiKey
	var streamSender func(*string, *string) error
	if len(sender) > 0 {
		streamSender = sender[0]
	}

	switch strings.ToLower(strings.TrimSpace(provider.Name)) {
	case "openai":
		return (&models.OpenAIModel{
			BaseModel: models.BaseModel{
				BaseURL:    strings.TrimRight(provider.BaseURL, "/"),
				URLSuffix:  strings.TrimLeft(provider.URLSuffix["chat"], "/"),
				HTTPClient: s.httpClient,
			},
		}).Chat(ctx, &modelCfg, &chatCfg, messages, modelName, streamSender)
	case "deepseek":
		return (&models.DeepSeekModel{
			BaseModel: models.BaseModel{
				BaseURL:    strings.TrimRight(provider.BaseURL, "/"),
				URLSuffix:  strings.TrimLeft(provider.URLSuffix["chat"], "/"),
				HTTPClient: s.httpClient,
			},
		}).Chat(ctx, &modelCfg, &chatCfg, messages, modelName, streamSender)
	case "moonshot":
		return (&models.MoonShotModel{
			BaseModel: models.BaseModel{
				BaseURL:    strings.TrimRight(provider.BaseURL, "/"),
				URLSuffix:  strings.TrimLeft(provider.URLSuffix["chat"], "/"),
				HTTPClient: s.httpClient,
			},
		}).Chat(ctx, &modelCfg, &chatCfg, messages, modelName, streamSender)
	case "siliconflow":
		return (&models.SiliconFlowModel{
			BaseModel: models.BaseModel{
				BaseURL:    strings.TrimRight(provider.BaseURL, "/"),
				URLSuffix:  strings.TrimLeft(provider.URLSuffix["chat"], "/"),
				HTTPClient: s.httpClient,
			},
		}).Chat(ctx, &modelCfg, &chatCfg, messages, modelName, streamSender)
	case "aliyun":
		return (&models.AliyunModel{
			BaseModel: models.BaseModel{
				BaseURL:    strings.TrimRight(provider.BaseURL, "/"),
				URLSuffix:  strings.TrimLeft(provider.URLSuffix["chat"], "/"),
				HTTPClient: s.httpClient,
			},
		}).Chat(ctx, &modelCfg, &chatCfg, messages, modelName, streamSender)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedProvider, provider.Name)
	}
}

// ListSupportedModels returns models supported by a provider configuration.
func (s *ModelService) ListSupportedModels(ctx context.Context, providerID string, modelCfg models.ModelConfig) (*[]dto.ListModelsResponse, error) {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return nil, ErrProviderIDRequired
	}

	provider, err := s.providerDAO.GetProviderByID(ctx, dao.DB, providerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProviderNotFound
		}
		return nil, err
	}

	apiKey := strings.TrimSpace(provider.APIKey)
	modelCfg.APIKey = &apiKey

	switch strings.ToLower(strings.TrimSpace(provider.Name)) {
	case "openai":
		return (&models.OpenAIModel{
			BaseModel: models.BaseModel{
				BaseURL:    strings.TrimRight(provider.BaseURL, "/"),
				URLSuffix:  strings.TrimLeft(provider.URLSuffix["models"], "/"),
				HTTPClient: s.httpClient,
			},
		}).ListModels(ctx, &modelCfg)
	case "deepseek":
		return (&models.DeepSeekModel{
			BaseModel: models.BaseModel{
				BaseURL:    strings.TrimRight(provider.BaseURL, "/"),
				URLSuffix:  strings.TrimLeft(provider.URLSuffix["models"], "/"),
				HTTPClient: s.httpClient,
			},
		}).ListModels(ctx, &modelCfg)
	case "moonshot":
		return (&models.MoonShotModel{
			BaseModel: models.BaseModel{
				BaseURL:    strings.TrimRight(provider.BaseURL, "/"),
				URLSuffix:  strings.TrimLeft(provider.URLSuffix["models"], "/"),
				HTTPClient: s.httpClient,
			},
		}).ListModels(ctx, &modelCfg)
	case "siliconflow":
		return (&models.SiliconFlowModel{
			BaseModel: models.BaseModel{
				BaseURL:    strings.TrimRight(provider.BaseURL, "/"),
				URLSuffix:  strings.TrimLeft(provider.URLSuffix["models"], "/"),
				HTTPClient: s.httpClient,
			},
		}).ListModels(ctx, &modelCfg)
	case "aliyun":
		return (&models.AliyunModel{
			BaseModel: models.BaseModel{
				BaseURL:    strings.TrimRight(provider.BaseURL, "/"),
				URLSuffix:  strings.TrimLeft(provider.URLSuffix["models"], "/"),
				HTTPClient: s.httpClient,
			},
		}).ListModels(ctx, &modelCfg)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedProvider, provider.Name)
	}
}
