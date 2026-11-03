package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mockdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func TestImportAds(t *testing.T) {
	ctx := context.Background()

	importAdsRequest := usecase.ImportAdsRequest{
		OwnerID:        "user-1",
		IdempotencyKey: "6f1c2b8e-3d4a-4f5b-9c6d-7e8f9a0b1c2d",
		Ads: []usecase.ImportedAd{
			{Title: "Mountain bike", PriceCents: 12000, PhotoURL: "https://example.com/photos/bike.jpg"},
			{Title: "Leather sofa", PriceCents: 25000, PhotoURL: "https://example.com/photos/sofa.jpg"},
			{Title: "Vintage camera", PriceCents: 8550, PhotoURL: "https://example.com/photos/camera.jpg"},
		},
	}

	t.Run("nominal", func(t *testing.T) {
		dao := mockdao.New(t)
		tx := mockdao.New(t)
		dao.On("Begin", ctx).Return(tx, nil).Once()
		tx.On("CreateAd", ctx, domain.AdDraft{Title: "Mountain bike", PriceCents: 12000, PhotoURL: "https://example.com/photos/bike.jpg", OwnerID: "user-1"}).Return(domain.AdID(10), nil).Once()
		tx.On("CreateAd", ctx, domain.AdDraft{Title: "Leather sofa", PriceCents: 25000, PhotoURL: "https://example.com/photos/sofa.jpg", OwnerID: "user-1"}).Return(domain.AdID(11), nil).Once()
		tx.On("CreateAd", ctx, domain.AdDraft{Title: "Vintage camera", PriceCents: 8550, PhotoURL: "https://example.com/photos/camera.jpg", OwnerID: "user-1"}).Return(domain.AdID(12), nil).Once()
		tx.On("Commit").Return(nil).Once()
		// The deferred Rollback runs even after a successful Commit, where it does nothing.
		tx.On("Rollback").Return(nil).Maybe()

		adIDs, err := usecase.NewImportAdsFunc(dao)(ctx, importAdsRequest)

		require.NoError(t, err)
		assert.Equal(t, []domain.AdID{10, 11, 12}, adIDs)
	})

	t.Run("returns the error when the transaction cannot begin", func(t *testing.T) {
		dao := mockdao.New(t)
		dbErr := errors.New("boom")
		dao.On("Begin", ctx).Return(nil, dbErr).Once()

		adIDs, err := usecase.NewImportAdsFunc(dao)(ctx, importAdsRequest)

		require.ErrorIs(t, err, dbErr)
		assert.Nil(t, adIDs)
	})

	t.Run("rolls back and returns the error when an ad cannot be created", func(t *testing.T) {
		dao := mockdao.New(t)
		tx := mockdao.New(t)
		dao.On("Begin", ctx).Return(tx, nil).Once()
		dbErr := errors.New("boom")
		tx.On("CreateAd", ctx, domain.AdDraft{Title: "Mountain bike", PriceCents: 12000, PhotoURL: "https://example.com/photos/bike.jpg", OwnerID: "user-1"}).Return(domain.AdID(10), nil).Once()
		tx.On("CreateAd", ctx, domain.AdDraft{Title: "Leather sofa", PriceCents: 25000, PhotoURL: "https://example.com/photos/sofa.jpg", OwnerID: "user-1"}).Return(domain.AdID(0), dbErr).Once()
		tx.On("Rollback").Return(nil).Once()

		adIDs, err := usecase.NewImportAdsFunc(dao)(ctx, importAdsRequest)

		tx.AssertNotCalled(t, "Commit")
		require.ErrorIs(t, err, dbErr)
		assert.Nil(t, adIDs)
	})

	t.Run("returns the error when the transaction cannot commit", func(t *testing.T) {
		dao := mockdao.New(t)
		tx := mockdao.New(t)
		dao.On("Begin", ctx).Return(tx, nil).Once()
		tx.On("CreateAd", ctx, domain.AdDraft{Title: "Mountain bike", PriceCents: 12000, PhotoURL: "https://example.com/photos/bike.jpg", OwnerID: "user-1"}).Return(domain.AdID(10), nil).Once()
		tx.On("CreateAd", ctx, domain.AdDraft{Title: "Leather sofa", PriceCents: 25000, PhotoURL: "https://example.com/photos/sofa.jpg", OwnerID: "user-1"}).Return(domain.AdID(11), nil).Once()
		tx.On("CreateAd", ctx, domain.AdDraft{Title: "Vintage camera", PriceCents: 8550, PhotoURL: "https://example.com/photos/camera.jpg", OwnerID: "user-1"}).Return(domain.AdID(12), nil).Once()
		dbErr := errors.New("boom")
		tx.On("Commit").Return(dbErr).Once()
		// A failed Commit already ends the transaction: rolling it back afterwards is allowed but not required.
		tx.On("Rollback").Return(nil).Maybe()

		adIDs, err := usecase.NewImportAdsFunc(dao)(ctx, importAdsRequest)

		require.ErrorIs(t, err, dbErr)
		assert.Nil(t, adIDs)
	})

	t.Run("stores the normalized title", func(t *testing.T) {
		dao := mockdao.New(t)
		tx := mockdao.New(t)
		dao.On("Begin", ctx).Return(tx, nil).Once()
		tx.On("CreateAd", ctx, domain.AdDraft{Title: "Mountain bike", PriceCents: 12000, PhotoURL: "https://example.com/photos/bike.jpg", OwnerID: "user-1"}).Return(domain.AdID(10), nil).Once()
		tx.On("Commit").Return(nil).Once()
		tx.On("Rollback").Return(nil).Maybe()

		adIDs, err := usecase.NewImportAdsFunc(dao)(ctx, usecase.ImportAdsRequest{
			OwnerID:        "user-1",
			IdempotencyKey: "6f1c2b8e-3d4a-4f5b-9c6d-7e8f9a0b1c2d",
			Ads: []usecase.ImportedAd{
				{Title: "  Mountain bike  ", PriceCents: 12000, PhotoURL: "https://example.com/photos/bike.jpg"},
			},
		})

		require.NoError(t, err)
		assert.Equal(t, []domain.AdID{10}, adIDs)
	})

	t.Run("rejects the whole import without opening a transaction when an ad is invalid", func(t *testing.T) {
		// The last ad is the invalid one, so that validating only the first ad would not be enough.
		invalidImport := usecase.ImportAdsRequest{
			OwnerID:        "user-1",
			IdempotencyKey: "6f1c2b8e-3d4a-4f5b-9c6d-7e8f9a0b1c2d",
			Ads: []usecase.ImportedAd{
				{Title: "Mountain bike", PriceCents: 12000, PhotoURL: "https://example.com/photos/bike.jpg"},
				{Title: "Leather sofa", PriceCents: 25000, PhotoURL: "https://example.com/photos/sofa.jpg"},
				{Title: "   ", PriceCents: 8550, PhotoURL: "https://example.com/photos/camera.jpg"},
			},
		}
		// No expectation: neither Begin nor CreateAd must be called.
		dao := mockdao.New(t)

		adIDs, err := usecase.NewImportAdsFunc(dao)(ctx, invalidImport)

		require.ErrorIs(t, err, domain.ErrInvalidAd)
		assert.Nil(t, adIDs)
	})
}
