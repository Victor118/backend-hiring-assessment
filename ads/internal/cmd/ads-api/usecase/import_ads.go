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
	// len covers both an empty ads array and a body without any ads field, which decodes into a nil slice.
	if len(req.Ads) == 0 {
		return nil, fmt.Errorf("%w: the import has no ads", domain.ErrInvalidAd)
	}

	drafts := make([]domain.AdDraft, 0, len(req.Ads))
	// Every ad is checked before the transaction opens: an invalid import is rejected as a whole, without touching the database.
	for i, ad := range req.Ads {
		d, err := domain.NewAdDraft(ad.Title, ad.PriceCents, ad.PhotoURL, req.OwnerID)
		if err != nil {
			return nil, fmt.Errorf("unable to import ad %d: %w", i, err)
		}
		drafts = append(drafts, d)
	}

	adImport := domain.AdImport{
		OwnerID: req.OwnerID,
		Drafts:  drafts,
		Key:     req.IdempotencyKey,
	}
	fingerprint := adImport.Fingerprint()

	tx, err := uc.adRepository.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to import ads: %w", err)
	}
	// Rolls back every ad created so far when the import stops early. It does nothing once the transaction is committed.
	defer func() { _ = tx.Rollback() }()

	// Reserving the key also locks it: a concurrent import with the same key waits until this transaction ends.
	record, found, err := tx.ReserveImport(ctx, adImport.OwnerID, adImport.Key, fingerprint)
	if err != nil {
		return nil, fmt.Errorf("unable to import ads: %w", err)
	}
	if found {
		return adImport.Replay(record)
	}

	results := make([]domain.AdID, 0, len(req.Ads))
	for i, draft := range drafts {
		id, err := tx.CreateAd(ctx, draft)
		if err != nil {
			return nil, fmt.Errorf("unable to import ad %d: %w", i, err)
		}

		results = append(results, id)
	}

	if err := tx.CompleteImport(ctx, adImport.OwnerID, adImport.Key, results); err != nil {
		return nil, fmt.Errorf("unable to complete the import: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("unable to commit the import: %w", err)
	}

	return results, nil
}
