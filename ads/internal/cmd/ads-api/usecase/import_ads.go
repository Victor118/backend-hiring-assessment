package usecase

import (
	"context"
	"fmt"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain/ports"
)

type (
	// ImportAdsFunc creates a batch of ads owned by the calling user, all or none of them, and returns their identifiers
	// in the order of the request.
	ImportAdsFunc func(ctx context.Context, req ImportAdsRequest) ([]domain.AdID, error)

	// ImportedAd describes one ad of an import. It has no owner: every ad of an import belongs to the caller.
	ImportedAd struct {
		Title      string
		PriceCents int64
		PhotoURL   string
	}

	// ImportAdsRequest describes an import. OwnerID is the authenticated user: a caller can never import ads on behalf
	// of someone else. IdempotencyKey identifies the import across retries, so that retrying it does not create the
	// ads twice.
	ImportAdsRequest struct {
		OwnerID        domain.UserID
		IdempotencyKey string
		Ads            []ImportedAd
	}

	importAds struct {
		adRepository ports.TransactionalAdRepository
	}
)

// NewImportAdsFunc returns an ImportAdsFunc backed by the given repository.
func NewImportAdsFunc(adRepository ports.TransactionalAdRepository) ImportAdsFunc {
	return importAds{
		adRepository: adRepository,
	}.importAds
}

func (uc importAds) importAds(ctx context.Context, req ImportAdsRequest) ([]domain.AdID, error) {
	tx, err := uc.adRepository.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to import ads: %w", err)
	}
	// Rolls back every ad created so far when the import stops early. It does nothing once the transaction is committed.
	defer func() { _ = tx.Rollback() }()

	results := make([]domain.AdID, 0, len(req.Ads))
	for i, importedAd := range req.Ads {
		newAd := ports.NewAd{
			Title:      importedAd.Title,
			PhotoURL:   importedAd.PhotoURL,
			PriceCents: importedAd.PriceCents,
			OwnerID:    req.OwnerID,
		}

		id, err := tx.CreateAd(ctx, newAd)
		if err != nil {
			return nil, fmt.Errorf("unable to import ad %d: %w", i, err)
		}

		results = append(results, id)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("unable to commit the import: %w", err)
	}

	return results, nil
}
