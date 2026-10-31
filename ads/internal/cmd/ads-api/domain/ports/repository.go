package ports

import (
	"context"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

// AdRepository persists and retrieves ads.
type AdRepository interface {
	// CreateAd inserts an ad and returns its generated identifier. It returns domain.ErrUnknownOwner when the owner does not exist.
	CreateAd(ctx context.Context, ad NewAd) (domain.AdID, error)
	// GetAdByID returns a single ad. It returns domain.ErrAdNotFound when no ad matches.
	GetAdByID(ctx context.Context, id domain.AdID) (domain.Ad, error)
	// SearchAds returns all ads matching the filter.
	SearchAds(ctx context.Context, filter AdFilter) ([]domain.Ad, error)
	// DeleteAd deletes a single ad. Deleting an ad that does not exist is not an error, so the call is idempotent.
	DeleteAd(ctx context.Context, id domain.AdID) error
}

// AdFilter describes an ad search. Zero values mean "no filter on this field".
type AdFilter struct {
	OwnerID       domain.UserID
	MinPriceCents int64
	MaxPriceCents int64
	Title         string
}

// NewAd carries the data needed to create an ad.
type NewAd struct {
	Title      string
	PriceCents int64
	PhotoURL   string
	OwnerID    domain.UserID
}
