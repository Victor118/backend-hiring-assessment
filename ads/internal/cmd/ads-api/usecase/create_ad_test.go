package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func TestCreateAd(t *testing.T) {
	ctx := context.Background()

	ucReq := usecase.CreateAdRequest{
		Title:      "mountain bike",
		PriceCents: 15000,
		PhotoURL:   "https://example.com/bike.jpg",
		OwnerID:    "user-1",
	}
	draft := domain.AdDraft{
		Title:      "mountain bike",
		PriceCents: 15000,
		PhotoURL:   "https://example.com/bike.jpg",
		OwnerID:    "user-1",
	}

	t.Run("nominal", func(t *testing.T) {
		m := mock.New(t)
		m.On("CreateAd", ctx, draft).Return(domain.AdID(7), nil)

		got, err := usecase.NewCreateAdFunc(m)(ctx, ucReq)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(7), got)
	})

	t.Run("error from dao", func(t *testing.T) {
		m := mock.New(t)
		dbErr := errors.New("connection reset")
		m.On("CreateAd", ctx, draft).Return(domain.AdID(0), dbErr)

		_, err := usecase.NewCreateAdFunc(m)(ctx, ucReq)

		require.Error(t, err)
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("accepts a free ad", func(t *testing.T) {
		free := ucReq
		free.PriceCents = 0
		freeDraft := draft
		freeDraft.PriceCents = 0

		m := mock.New(t)
		m.On("CreateAd", ctx, freeDraft).Return(domain.AdID(8), nil)

		got, err := usecase.NewCreateAdFunc(m)(ctx, free)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(8), got)
	})

	t.Run("stores the normalized title", func(t *testing.T) {
		padded := ucReq
		padded.Title = "  mountain bike  "

		m := mock.New(t)
		m.On("CreateAd", ctx, draft).Return(domain.AdID(7), nil)

		got, err := usecase.NewCreateAdFunc(m)(ctx, padded)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(7), got)
	})

	invalidRequests := map[string]func(req *usecase.CreateAdRequest){
		"empty title":           func(req *usecase.CreateAdRequest) { req.Title = "" },
		"whitespace-only title": func(req *usecase.CreateAdRequest) { req.Title = "   " },
		"negative price":        func(req *usecase.CreateAdRequest) { req.PriceCents = -1 },
	}
	for name, invalidate := range invalidRequests {
		t.Run("rejects an invalid ad: "+name, func(t *testing.T) {
			req := ucReq
			invalidate(&req)
			// No expectation: the repository must not be called with an invalid ad.
			m := mock.New(t)

			_, err := usecase.NewCreateAdFunc(m)(ctx, req)

			require.ErrorIs(t, err, domain.ErrInvalidAd)
		})
	}
}
