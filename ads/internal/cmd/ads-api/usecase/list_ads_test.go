package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain/ports"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func TestListAds(t *testing.T) {
	ctx := context.Background()

	t.Run("nominal", func(t *testing.T) {
		t.Run("returns all matching ads", func(t *testing.T) {
			m := mock.New(t)
			want := []domain.Ad{
				{ID: 1, Title: "bike", Price: 5000, UserID: "user-1"},
				{ID: 2, Title: "scooter", Price: 12000, UserID: "user-2"},
			}
			req := ports.AdFilter{Title: "bike"}
			m.On("SearchAds", ctx, req).Return(want, nil)

			got, err := usecase.NewListAdsFunc(m)(ctx, usecase.ListAdsRequest{Title: "bike"})

			require.NoError(t, err)
			assert.Equal(t, want, got)
		})

		t.Run("returns empty slice when no ads match", func(t *testing.T) {
			m := mock.New(t)
			m.On("SearchAds", ctx, ports.AdFilter{Title: "ufo"}).Return(nil, nil)

			got, err := usecase.NewListAdsFunc(m)(ctx, usecase.ListAdsRequest{Title: "ufo"})

			require.NoError(t, err)
			assert.Nil(t, got)
		})
	})

	t.Run("wraps repository errors", func(t *testing.T) {
		m := mock.New(t)
		dbErr := errors.New("connection reset")
		m.On("SearchAds", ctx, ports.AdFilter{}).Return(nil, dbErr)

		_, err := usecase.NewListAdsFunc(m)(ctx, usecase.ListAdsRequest{})

		require.Error(t, err)
		assert.ErrorIs(t, err, dbErr)
	})
}
