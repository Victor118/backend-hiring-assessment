package ports

import (
	"context"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

// Identities resolves information about users.
type Identities interface {
	// GetUsername returns the username of a user. It returns domain.ErrUserNotFound when the user does not exist.
	GetUsername(ctx context.Context, userID domain.UserID) (string, error)
}
