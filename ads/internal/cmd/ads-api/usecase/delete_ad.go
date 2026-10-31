package usecase

import (
	"context"
	"fmt"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain/ports"
)

type (
	// DeleteAdFunc deletes an ad. It fails with domain.ErrAdNotFound when no ad matches the identifier.
	DeleteAdFunc func(ctx context.Context, req DeleteAdRequest) error

	// DeleteAdRequest describes the ID of the ad to delete.
	DeleteAdRequest struct {
		ID domain.AdID
	}

	deleteAd struct {
		adRepository ports.AdRepository
	}
)

// NewDeleteAdFunc returns a DeleteAdFunc backed by the given repository.
func NewDeleteAdFunc(adRepository ports.AdRepository) DeleteAdFunc {
	return deleteAd{adRepository: adRepository}.deleteAd
}

func (uc deleteAd) deleteAd(ctx context.Context, req DeleteAdRequest) error {
	_, err := uc.adRepository.GetAdByID(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("unable to get ad %s: %w", req.ID, err)
	}

	err = uc.adRepository.DeleteAd(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("unable to delete ad %s: %w", req.ID, err)
	}

	return nil
}
