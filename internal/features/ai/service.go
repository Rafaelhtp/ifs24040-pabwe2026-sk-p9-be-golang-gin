package ai

import (
	"context"
	"errors"
	"fmt"

	"app/internal/features/post"
)

var (
	ErrInvalidCategory = errors.New("kategori tidak valid")
	ErrSearchFailed    = errors.New("gagal mengambil hasil pencarian web")
	ErrAIFailed        = errors.New("gagal membuat artikel dengan AI")
)

type Service struct {
	search *SearchClient
	llm    *OpenAIClient
}

func NewService(search *SearchClient, llm *OpenAIClient) *Service {
	return &Service{search: search, llm: llm}
}

func isValidCategory(category string) bool {
	for _, c := range post.Categories {
		if c == category {
			return true
		}
	}
	return false
}

func (s *Service) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	category := req.Category
	if category == "" {
		category = "Umum"
	}
	if !isValidCategory(category) {
		return nil, ErrInvalidCategory
	}

	sources := []SearchSource{}
	if req.IncludeWebSearch {
		found, err := s.search.Search(ctx, req.Topic)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrSearchFailed, err)
		}
		sources = append(sources, found...)
	}

	article, err := s.llm.Generate(ctx, req.Topic, category, sources)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAIFailed, err)
	}
	if !isValidCategory(article.Category) {
		article.Category = category
	}

	return &GenerateResponse{Article: *article, SearchSources: sources}, nil
}
