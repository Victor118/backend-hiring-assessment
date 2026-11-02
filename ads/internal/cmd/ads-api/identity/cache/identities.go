package cache

import (
	"context"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain/ports"
)

var _ ports.Identities = (*Identities)(nil)

// Identities is a ports.Identities decorator that keeps resolved usernames in memory, so that the heavily read
// ad endpoints do not call the users service on every request. Errors are never cached: a failing call is retried
// on the next request.
type Identities struct {
	inner     ports.Identities
	usernames *expirable.LRU[domain.UserID, string]
}

// New returns an Identities that delegates cache misses to inner. It holds at most size usernames, each for at most
// ttl, which bounds both its memory footprint and how long a renamed user keeps their former username.
func New(inner ports.Identities, size int, ttl time.Duration) *Identities {
	return &Identities{
		inner:     inner,
		usernames: expirable.NewLRU[domain.UserID, string](size, nil, ttl),
	}
}

// GetUsername returns the cached username of the given user, or asks the inner Identities on a miss.
func (i *Identities) GetUsername(ctx context.Context, userID domain.UserID) (string, error) {
	if username, ok := i.usernames.Get(userID); ok {
		return username, nil
	}

	username, err := i.inner.GetUsername(ctx, userID)
	if err != nil {
		return "", err
	}

	i.usernames.Add(userID, username)

	return username, nil
}
