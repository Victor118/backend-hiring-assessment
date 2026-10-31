package usecase

import (
	"context"
	"fmt"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain/ports"
)

type (
	// ListAdsFunc returns all ads matching the filters.
	ListAdsFunc func(ctx context.Context, req ListAdsRequest) ([]domain.Ad, error)

	// ListAdsRequest describes which ads to return.
	ListAdsRequest struct {
		OwnerID       domain.UserID
		MinPriceCents int64
		MaxPriceCents int64
		Title         string
	}

	listAds struct {
		adRepository ports.AdRepository
	}
)

// NewListAdsFunc returns a new ListAdsFunc.
func NewListAdsFunc(adRepository ports.AdRepository) ListAdsFunc {
	return listAds{adRepository: adRepository}.listAds
}

func (uc listAds) listAds(ctx context.Context, req ListAdsRequest) ([]domain.Ad, error) {
	ads, err := uc.adRepository.SearchAds(ctx, ports.AdFilter{
		OwnerID:       req.OwnerID,
		MinPriceCents: req.MinPriceCents,
		MaxPriceCents: req.MaxPriceCents,
		Title:         req.Title,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to search ads: %w", err)
	}

	return ads, nil
}
