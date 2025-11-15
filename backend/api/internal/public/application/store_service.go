package application

import (
	"context"

	"github.com/sngm3741/makoto-club-services/api/internal/public/domain"
)

// storeQueryService is the concrete implementation of StoreQueryService.
type storeQueryService struct {
	repo StoreRepository
}

// NewStoreQueryService creates a new store query service.
func NewStoreQueryService(repo StoreRepository) StoreQueryService {
	return &storeQueryService{repo: repo}
}

func (s *storeQueryService) List(ctx context.Context, filter StoreFilter, paging Paging) ([]domain.Store, error) {
	return s.repo.Find(ctx, filter, paging)
}

func (s *storeQueryService) Detail(ctx context.Context, id string) (*domain.Store, error) {
	return s.repo.FindByID(ctx, id)
}
