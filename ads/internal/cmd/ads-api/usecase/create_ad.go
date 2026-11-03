package usecase

import (
	"context"
	"fmt"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain/ports"
)

type (
	// CreateAdFunc creates an ad owned by the calling user.
	CreateAdFunc func(ctx context.Context, req CreateAdRequest) (domain.AdID, error)

	// CreateAdRequest describes the ad to create. OwnerID is the authenticated user: a caller can never create an ad on behalf of someone else.
	CreateAdRequest struct {
		Title      string
		PriceCents int64
		PhotoURL   string
		OwnerID    domain.UserID
	}

	createAd struct {
		adRepository ports.AdRepository
	}
)

// NewCreateAdFunc returns a CreateAdFunc backed by the given repository.
func NewCreateAdFunc(adRepository ports.AdRepository) CreateAdFunc {
	return createAd{adRepository: adRepository}.createAd
}

func (uc createAd) createAd(ctx context.Context, req CreateAdRequest) (domain.AdID, error) {
	draft, err := domain.NewAdDraft(req.Title, req.PriceCents, req.PhotoURL, req.OwnerID)
	if err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}
	adID, err := uc.adRepository.CreateAd(ctx, draft)
	if err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	return adID, nil
}
