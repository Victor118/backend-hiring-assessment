package ports

import (
	"context"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

// AdRepository persists and retrieves ads.
type AdRepository interface {
	// CreateAd inserts an ad and returns its generated identifier. It returns domain.ErrUnknownOwner when the owner does not exist.
	CreateAd(ctx context.Context, ad domain.AdDraft) (domain.AdID, error)
	// GetAdByID returns a single ad. It returns domain.ErrAdNotFound when no ad matches.
	GetAdByID(ctx context.Context, id domain.AdID) (domain.Ad, error)
	// SearchAds returns all ads matching the filter.
	SearchAds(ctx context.Context, filter AdFilter) ([]domain.Ad, error)
	// DeleteAd deletes a single ad. Deleting an ad that does not exist is not an error, so the call is idempotent.
	DeleteAd(ctx context.Context, id domain.AdID) error
}

// TransactionalAdRepository is an AdRepository that can open transactions. Every call made directly on it is its own transaction.
type TransactionalAdRepository interface {
	AdRepository
	// Begin opens a transaction. Every call made on the returned AdTx is part of it, and the caller must end it with Commit or Rollback.
	Begin(ctx context.Context) (AdTx, error)
}

// AdTx is an AdRepository scoped to an opened transaction: every call made on it is part of that transaction.
type AdTx interface {
	AdRepository
	// ReserveImport claims the import key for the owner within the transaction, recording the fingerprint of the
	// import. When the key was already used, it reserves nothing and returns the stored import with found set to true.
	// A concurrent transaction reserving the same key waits until this one ends.
	ReserveImport(ctx context.Context, ownerID domain.UserID, key string, requestHash []byte) (previous domain.ImportRecord, found bool, err error)
	// CompleteImport stores, under an import key reserved within the transaction, the identifiers of the ads the import
	// created, so that a retry can be answered with them.
	CompleteImport(ctx context.Context, ownerID domain.UserID, key string, adIDs []domain.AdID) error
	// Commit applies every change made through the transaction.
	Commit() error
	// Rollback discards every change made through the transaction. Calling it on an already committed transaction does nothing, so `defer func() { _ = tx.Rollback() }()` is a safe guard.
	Rollback() error
}

// AdFilter describes an ad search. Zero values mean "no filter on this field".
type AdFilter struct {
	OwnerID       domain.UserID
	MinPriceCents int64
	MaxPriceCents int64
	Title         string
}
