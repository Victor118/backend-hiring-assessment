package usecase

import (
	"context"
	"fmt"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

type (
	// DeleteAdFunc deletes an ad. It fails with domain.ErrAdNotFound when no ad matches the identifier.
	DeleteAdFunc func(ctx context.Context, req DeleteAdRequest) error

	// DeleteAdRequest describes the ID of the ad to delete.
	DeleteAdRequest struct {
		ID domain.AdID
	}

	deleteAd struct {
		adDAO adsdao.DAO
	}
)

// NewDeleteAdFunc returns a DeleteAdFunc backed by the given DAO.
func NewDeleteAdFunc(adDAO adsdao.DAO) DeleteAdFunc {
	return deleteAd{adDAO: adDAO}.deleteAd
}

func (uc deleteAd) deleteAd(ctx context.Context, req DeleteAdRequest) error {
	_, err := uc.adDAO.GetAdByID(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("unable to get ad %s: %w", req.ID, err)
	}

	err = uc.adDAO.DeleteAd(ctx, adsdao.DeleteAdRequest{ID: req.ID})
	if err != nil {
		return fmt.Errorf("unable to delete ad %s: %w", req.ID, err)
	}

	return nil
}
