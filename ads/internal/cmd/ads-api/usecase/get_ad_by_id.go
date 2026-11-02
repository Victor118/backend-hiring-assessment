package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain/ports"
)

type (
	// GetAdByIDFunc returns a single ad along with its owner's username. It fails with domain.ErrAdNotFound when no ad matches the identifier.
	// The username is left empty when it cannot be resolved, so that the ad stays available when the users service is not.
	GetAdByIDFunc func(ctx context.Context, id domain.AdID) (domain.AdDetails, error)

	getAdByID struct {
		adRepository ports.AdRepository
		identities   ports.Identities
		logger       *slog.Logger
	}
)

// NewGetAdByIDFunc returns a GetAdByIDFunc that reads ads from the given repository and usernames from the given identities.
// Failures to resolve a username are reported to the given logger.
func NewGetAdByIDFunc(adRepository ports.AdRepository, identities ports.Identities, logger *slog.Logger) GetAdByIDFunc {
	return getAdByID{
		adRepository: adRepository,
		identities:   identities,
		logger:       logger,
	}.getAdByID
}

func (uc getAdByID) getAdByID(ctx context.Context, id domain.AdID) (domain.AdDetails, error) {
	found, err := uc.adRepository.GetAdByID(ctx, id)
	if err != nil {
		return domain.AdDetails{}, fmt.Errorf("unable to get ad %s: %w", id, err)
	}

	username, err := uc.identities.GetUsername(ctx, found.UserID)
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		uc.logger.WarnContext(ctx, "ad owner does not exist, returning the ad without username",
			"ad_id", found.ID, "user_id", found.UserID, "error", err)

		username = ""
	case err != nil:
		uc.logger.ErrorContext(ctx, "unable to get the ad owner's username, returning the ad without it",
			"ad_id", found.ID, "user_id", found.UserID, "error", err)

		username = ""
	}

	return domain.AdDetails{
		Ad:       found,
		Username: username,
	}, nil
}
