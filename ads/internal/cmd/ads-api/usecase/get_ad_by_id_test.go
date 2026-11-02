package usecase_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	mockidentity "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/identity/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func TestGetAdByID(t *testing.T) {
	ctx := context.Background()

	t.Run("nominal", func(t *testing.T) {
		m := mockdao.New(t)
		identities := mockidentity.New(t)
		ad := domain.Ad{ID: 42, Title: "bike", Price: 5000, UserID: "user-1"}
		m.On("GetAdByID", ctx, domain.AdID(42)).Return(ad, nil)
		username := "alice"
		identities.On("GetUsername", ctx, domain.UserID("user-1")).Return(username, nil)
		want := domain.AdDetails{Ad: ad, Username: username}

		got, err := usecase.NewGetAdByIDFunc(m, identities, slog.New(slog.DiscardHandler))(ctx, 42)

		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("error no ad found", func(t *testing.T) {
		m := mockdao.New(t)
		identities := mockidentity.New(t)
		m.On("GetAdByID", ctx, domain.AdID(99)).Return(domain.Ad{}, domain.ErrAdNotFound)

		_, err := usecase.NewGetAdByIDFunc(m, identities, slog.New(slog.DiscardHandler))(ctx, 99)

		require.Error(t, err)
		identities.AssertNotCalled(t, "GetUsername", mock.Anything, mock.Anything)
		assert.ErrorIs(t, err, domain.ErrAdNotFound)
	})

	t.Run("error from dao", func(t *testing.T) {
		m := mockdao.New(t)
		identities := mockidentity.New(t)
		dbErr := errors.New("connection reset")
		m.On("GetAdByID", ctx, domain.AdID(1)).Return(domain.Ad{}, dbErr)

		_, err := usecase.NewGetAdByIDFunc(m, identities, slog.New(slog.DiscardHandler))(ctx, 1)

		identities.AssertNotCalled(t, "GetUsername", mock.Anything, mock.Anything)
		assert.ErrorIs(t, err, dbErr)
	})

	usernameFailures := []struct {
		name      string
		usersErr  error
		wantLevel string
	}{
		{name: "returns the ad without username and logs an error when users service fails", usersErr: errors.New("boom"), wantLevel: "ERROR"},
		{name: "returns the ad without username and logs a warning when the owner is unknown", usersErr: domain.ErrUserNotFound, wantLevel: "WARN"},
	}

	for _, tc := range usernameFailures {
		t.Run(tc.name, func(t *testing.T) {
			m := mockdao.New(t)
			identities := mockidentity.New(t)
			ad := domain.Ad{ID: 42, Title: "bike", Price: 5000, UserID: "user-1"}
			m.On("GetAdByID", ctx, domain.AdID(42)).Return(ad, nil)
			// Callers must not trust a value returned along with an error, hence the non-empty username.
			identities.On("GetUsername", ctx, domain.UserID("user-1")).Return("garbage", tc.usersErr)
			want := domain.AdDetails{Ad: ad, Username: ""}

			var buf bytes.Buffer

			logger := slog.New(slog.NewJSONHandler(&buf, nil))

			got, err := usecase.NewGetAdByIDFunc(m, identities, logger)(ctx, 42)

			require.NoError(t, err)
			assert.Equal(t, want, got)

			var entry struct {
				Level  string `json:"level"`
				UserID string `json:"user_id"`
				AdID   int64  `json:"ad_id"`
			}
			require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
			assert.Equal(t, tc.wantLevel, entry.Level)
			assert.Equal(t, "user-1", entry.UserID)
			assert.Equal(t, int64(42), entry.AdID)
		})
	}
}
