package usecase

import (
	"context"
	"fmt"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain/ports"
)

type (
	// CreateAdFunc creates an ad owned by the calling user. It returns domain.ErrQuotaExceeded when the user has already
	// published domain.PublicationQuota ads over the last 24 hours. Imported ads are not created through it, so they do
	// not count towards the quota.
	CreateAdFunc func(ctx context.Context, req CreateAdRequest) (domain.AdID, error)

	// CreateAdRequest describes the ad to create. OwnerID is the authenticated user: a caller can never create an ad on behalf of someone else.
	CreateAdRequest struct {
		Title      string
		PriceCents int64
		PhotoURL   string
		OwnerID    domain.UserID
	}

	createAd struct {
		adRepository ports.TransactionalAdRepository
	}
)

// NewCreateAdFunc returns a CreateAdFunc backed by the given repository.
func NewCreateAdFunc(adRepository ports.TransactionalAdRepository) CreateAdFunc {
	return createAd{adRepository: adRepository}.createAd
}

func (uc createAd) createAd(ctx context.Context, req CreateAdRequest) (domain.AdID, error) {
	draft, err := domain.NewAdDraft(req.Title, req.PriceCents, req.PhotoURL, req.OwnerID)
	if err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	tx, err := uc.adRepository.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}
	// Does nothing once the transaction is committed.
	defer func() { _ = tx.Rollback() }()

	// The publisher is locked before it is loaded: two concurrent creations would otherwise both see room left.
	if err := tx.LockPublisher(ctx, req.OwnerID); err != nil {
		return 0, fmt.Errorf("unable to lock the publisher: %w", err)
	}

	publisher, err := tx.LoadPublisher(ctx, req.OwnerID, domain.PublicationWindow)
	if err != nil {
		return 0, fmt.Errorf("unable to load the publisher: %w", err)
	}

	if err := publisher.AuthorizePublication(); err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	adID, err := tx.CreateAd(ctx, draft)
	if err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	if err := tx.RecordPublication(ctx, req.OwnerID); err != nil {
		return 0, fmt.Errorf("unable to record the publication: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("unable to commit the ad: %w", err)
	}

	return adID, nil
}
