package usecase

import (
	"context"
	"fmt"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain/ports"
)

type (
	// GetAdByIDFunc returns a single ad. It fails with ErrorCodeNotFound when no ad matches the identifier.
	GetAdByIDFunc func(ctx context.Context, id domain.AdID) (domain.Ad, error)

	getAdByID struct {
		adRepository ports.AdRepository
	}
)

// NewGetAdByIDFunc returns a GetAdByIDFunc backed by the given repository.
func NewGetAdByIDFunc(adRepository ports.AdRepository) GetAdByIDFunc {
	return getAdByID{adRepository: adRepository}.getAdByID
}

func (uc getAdByID) getAdByID(ctx context.Context, id domain.AdID) (domain.Ad, error) {
	found, err := uc.adRepository.GetAdByID(ctx, id)
	if err != nil {
		return domain.Ad{}, fmt.Errorf("unable to get ad %s: %w", id, err)
	}

	return found, nil
}
