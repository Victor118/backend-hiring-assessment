package cache_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/identity/cache"
	mockidentity "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/identity/mock"
)

func TestGetUsername(t *testing.T) {
	ctx := context.Background()

	t.Run("calls the inner identities on a miss", func(t *testing.T) {
		inner := mockidentity.New(t)
		inner.On("GetUsername", ctx, domain.UserID("user-1")).Return("alice", nil).Once()
		identities := cache.New(inner, 10, time.Minute)

		username, err := identities.GetUsername(ctx, "user-1")

		require.NoError(t, err)
		assert.Equal(t, "alice", username)
	})

	t.Run("serves a known user from the cache without calling the inner identities again", func(t *testing.T) {
		inner := mockidentity.New(t)
		inner.On("GetUsername", ctx, domain.UserID("user-1")).Return("alice", nil).Once()
		identities := cache.New(inner, 10, time.Minute)

		first, err := identities.GetUsername(ctx, "user-1")
		require.NoError(t, err)

		second, err := identities.GetUsername(ctx, "user-1")

		require.NoError(t, err)
		assert.Equal(t, "alice", first)
		assert.Equal(t, "alice", second)
		inner.AssertNumberOfCalls(t, "GetUsername", 1)
	})

	t.Run("caches each user under its own key", func(t *testing.T) {
		inner := mockidentity.New(t)
		inner.On("GetUsername", ctx, domain.UserID("user-1")).Return("alice", nil).Once()
		inner.On("GetUsername", ctx, domain.UserID("user-2")).Return("bob", nil).Once()
		identities := cache.New(inner, 10, time.Minute)

		first, err := identities.GetUsername(ctx, "user-1")
		require.NoError(t, err)

		second, err := identities.GetUsername(ctx, "user-2")

		require.NoError(t, err)
		assert.Equal(t, "alice", first)
		assert.Equal(t, "bob", second)
	})

	t.Run("does not cache errors so that the next call retries", func(t *testing.T) {
		inner := mockidentity.New(t)
		errBoom := errors.New("boom")
		inner.On("GetUsername", ctx, domain.UserID("user-1")).Return("", errBoom).Once()
		inner.On("GetUsername", ctx, domain.UserID("user-1")).Return("alice", nil).Once()
		identities := cache.New(inner, 10, time.Minute)

		_, err := identities.GetUsername(ctx, "user-1")
		require.ErrorIs(t, err, errBoom)

		username, err := identities.GetUsername(ctx, "user-1")

		require.NoError(t, err)
		assert.Equal(t, "alice", username)
	})

	t.Run("propagates ErrUserNotFound from the inner identities", func(t *testing.T) {
		inner := mockidentity.New(t)
		inner.On("GetUsername", ctx, domain.UserID("ghost")).Return("", domain.ErrUserNotFound).Once()
		identities := cache.New(inner, 10, time.Minute)

		_, err := identities.GetUsername(ctx, "ghost")

		require.ErrorIs(t, err, domain.ErrUserNotFound)
	})
}
