package service

import (
	"FunPDF/internal/dao"
	"FunPDF/internal/dto"
	"FunPDF/internal/engine"
	"FunPDF/internal/entity"
	"context"
	"database/sql"
	"errors"
	"strings"
)

type TranslatorService struct {
	translatorDAO *dao.TranslatorDAO
	translatorFct *engine.TranslatorFactory
}

func NewTranslatorService() TranslatorService {
	return TranslatorService{dao.NewTranslatorDAO(), engine.NewTranslatorFactory()}
}

// ListTranslators list all translators
func (s *TranslatorService) ListTranslators(ctx context.Context) ([]*entity.Translator, error) {
	list, err := s.translatorDAO.ListTranslators(ctx, dao.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []*entity.Translator{}, nil
		}
		return nil, err
	}
	return list, nil
}

// CreateTranslator create a unique translators
func (s *TranslatorService) CreateTranslator(ctx context.Context, req *dto.CreateTranslatorsRequest) (*entity.Translator, error) {
	translatorName := strings.ToLower(strings.TrimSpace(req.Name))
	if translatorName == "" {
		return nil, errors.New("translators name is required")
	}

	if len(req.Params) == 0 {
		return nil, errors.New("translators params is required")
	}

	translator, err := s.translatorDAO.CreateTranslator(ctx, dao.DB, translatorName, req.Params)
	if err != nil {
		return nil, err
	}
	return translator, nil
}

// Translate source to dst language
func (s *TranslatorService) Translate(ctx context.Context, req *dto.TranslateRequest, translatorName string) (string, error) {
	var from, to, query string
	if req.From == nil || strings.TrimSpace(*req.From) == "" {
		switch translatorName {
		case "baidu":
			from = "auto"
		default:
			from = ""
		}
	} else {
		from = strings.TrimSpace(*req.From)
	}
	if req.To == nil || strings.TrimSpace(*req.To) == "" {
		to = "zh"
	} else {
		to = strings.TrimSpace(*req.To)
	}

	if req.Q == nil {
		return "", nil
	}
	query = strings.TrimSpace(*req.Q)
	if query == "" {
		return "", nil
	}

	region := "default"
	if req.Region != nil && strings.TrimSpace(*req.Region) != "" {
		region = strings.TrimSpace(*req.Region)
	}

	translator, err := s.translatorFct.GetTranslator(ctx, dao.DB, translatorName, region)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrTranslatorNotFound
		}
		return "", err
	}

	result, err := translator.Translate(ctx, from, to, query, req.Params)
	if err != nil {
		return "", err
	}
	return result, nil
}
