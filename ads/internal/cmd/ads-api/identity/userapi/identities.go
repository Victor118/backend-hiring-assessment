package userapi

import (
	"context"
	"errors"
	"fmt"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain/ports"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/api"
)

var _ ports.Identities = (*Identities)(nil)

// Identities is a ports.Identities backed by the users service HTTP API.
type Identities struct {
	client *api.Client
}

// New returns an Identities calling the users service through the given client.
func New(client *api.Client) *Identities {
	return &Identities{
		client: client,
	}
}

// GetUsername returns the nickname of the given user. It returns domain.ErrUserNotFound when the users service does not know the user.
func (i *Identities) GetUsername(ctx context.Context, userID domain.UserID) (string, error) {
	user, err := i.client.GetUserByID(ctx, userID.String())
	if errors.Is(err, api.ErrNotFound) {
		return "", domain.ErrUserNotFound
	}

	if err != nil {
		return "", fmt.Errorf("unable to get user %s: %w", userID, err)
	}

	return user.Nickname, nil
}
