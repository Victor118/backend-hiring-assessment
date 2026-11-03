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

// importKey is the idempotency key of the imports under test.
const importKey = "6f1c2b8e-3d4a-4f5b-9c6d-7e8f9a0b1c2d"

// expectNewImport expects the reservation of a key that was never used, for the given import: the exact owner, key and
// fingerprint, so that a usecase reserving the wrong key fails the test.
func expectNewImport(ctx context.Context, tx *mockdao.Mock, adImport domain.AdImport) {
	tx.On("ReserveImport", ctx, adImport.OwnerID, adImport.Key, adImport.Fingerprint()).
		Return(domain.ImportRecord{}, false, nil).Once()
}

func TestImportAds(t *testing.T) {
	ctx := context.Background()

	importAdsRequest := usecase.ImportAdsRequest{
		OwnerID:        "user-1",
		IdempotencyKey: importKey,
		Ads: []usecase.ImportedAd{
			{Title: "Mountain bike", PriceCents: 12000, PhotoURL: "https://example.com/photos/bike.jpg"},
			{Title: "Leather sofa", PriceCents: 25000, PhotoURL: "https://example.com/photos/sofa.jpg"},
			{Title: "Vintage camera", PriceCents: 8550, PhotoURL: "https://example.com/photos/camera.jpg"},
		},
	}
	// adImport is the import built from importAdsRequest: the same drafts, with the title already normalized.
	adImport := domain.AdImport{
		OwnerID: "user-1",
		Key:     importKey,
		Drafts: []domain.AdDraft{
			{Title: "Mountain bike", PriceCents: 12000, PhotoURL: "https://example.com/photos/bike.jpg", OwnerID: "user-1"},
			{Title: "Leather sofa", PriceCents: 25000, PhotoURL: "https://example.com/photos/sofa.jpg", OwnerID: "user-1"},
			{Title: "Vintage camera", PriceCents: 8550, PhotoURL: "https://example.com/photos/camera.jpg", OwnerID: "user-1"},
		},
	}

	t.Run("nominal", func(t *testing.T) {
		dao := mockdao.New(t)
		tx := mockdao.New(t)
		dao.On("Begin", ctx).Return(tx, nil).Once()
		expectNewImport(ctx, tx, adImport)
		tx.On("CreateAd", ctx, domain.AdDraft{Title: "Mountain bike", PriceCents: 12000, PhotoURL: "https://example.com/photos/bike.jpg", OwnerID: "user-1"}).Return(domain.AdID(10), nil).Once()
		tx.On("CreateAd", ctx, domain.AdDraft{Title: "Leather sofa", PriceCents: 25000, PhotoURL: "https://example.com/photos/sofa.jpg", OwnerID: "user-1"}).Return(domain.AdID(11), nil).Once()
		tx.On("CreateAd", ctx, domain.AdDraft{Title: "Vintage camera", PriceCents: 8550, PhotoURL: "https://example.com/photos/camera.jpg", OwnerID: "user-1"}).Return(domain.AdID(12), nil).Once()
		// The IDs are stored under the key before the commit, so that they are committed together with the ads.
		complete := tx.On("CompleteImport", ctx, adImport.OwnerID, adImport.Key, []domain.AdID{10, 11, 12}).Return(nil).Once()
		tx.On("Commit").Return(nil).Once().NotBefore(complete)
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
		expectNewImport(ctx, tx, adImport)
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
		expectNewImport(ctx, tx, adImport)
		tx.On("CreateAd", ctx, domain.AdDraft{Title: "Mountain bike", PriceCents: 12000, PhotoURL: "https://example.com/photos/bike.jpg", OwnerID: "user-1"}).Return(domain.AdID(10), nil).Once()
		tx.On("CreateAd", ctx, domain.AdDraft{Title: "Leather sofa", PriceCents: 25000, PhotoURL: "https://example.com/photos/sofa.jpg", OwnerID: "user-1"}).Return(domain.AdID(11), nil).Once()
		tx.On("CreateAd", ctx, domain.AdDraft{Title: "Vintage camera", PriceCents: 8550, PhotoURL: "https://example.com/photos/camera.jpg", OwnerID: "user-1"}).Return(domain.AdID(12), nil).Once()
		tx.On("CompleteImport", ctx, adImport.OwnerID, adImport.Key, []domain.AdID{10, 11, 12}).Return(nil).Once()
		dbErr := errors.New("boom")
		tx.On("Commit").Return(dbErr).Once()
		// A failed Commit already ends the transaction: rolling it back afterwards is allowed but not required.
		tx.On("Rollback").Return(nil).Maybe()

		adIDs, err := usecase.NewImportAdsFunc(dao)(ctx, importAdsRequest)

		require.ErrorIs(t, err, dbErr)
		assert.Nil(t, adIDs)
	})

	t.Run("rolls back and returns the error when the import cannot be completed", func(t *testing.T) {
		dao := mockdao.New(t)
		tx := mockdao.New(t)
		dao.On("Begin", ctx).Return(tx, nil).Once()
		expectNewImport(ctx, tx, adImport)
		tx.On("CreateAd", ctx, domain.AdDraft{Title: "Mountain bike", PriceCents: 12000, PhotoURL: "https://example.com/photos/bike.jpg", OwnerID: "user-1"}).Return(domain.AdID(10), nil).Once()
		tx.On("CreateAd", ctx, domain.AdDraft{Title: "Leather sofa", PriceCents: 25000, PhotoURL: "https://example.com/photos/sofa.jpg", OwnerID: "user-1"}).Return(domain.AdID(11), nil).Once()
		tx.On("CreateAd", ctx, domain.AdDraft{Title: "Vintage camera", PriceCents: 8550, PhotoURL: "https://example.com/photos/camera.jpg", OwnerID: "user-1"}).Return(domain.AdID(12), nil).Once()
		dbErr := errors.New("boom")
		tx.On("CompleteImport", ctx, adImport.OwnerID, adImport.Key, []domain.AdID{10, 11, 12}).Return(dbErr).Once()
		tx.On("Rollback").Return(nil).Once()

		adIDs, err := usecase.NewImportAdsFunc(dao)(ctx, importAdsRequest)

		// Without the stored IDs a retry could not be replayed: the ads must not be committed either.
		tx.AssertNotCalled(t, "Commit")
		require.ErrorIs(t, err, dbErr)
		assert.Nil(t, adIDs)
	})

	t.Run("rejects a key already used for another content without creating any ad", func(t *testing.T) {
		otherContent := domain.AdImport{
			OwnerID: "user-1",
			Key:     importKey,
			Drafts:  []domain.AdDraft{{Title: "Old bike", PriceCents: 5000, PhotoURL: "https://example.com/photos/old.jpg", OwnerID: "user-1"}},
		}
		dao := mockdao.New(t)
		tx := mockdao.New(t)
		dao.On("Begin", ctx).Return(tx, nil).Once()
		tx.On("ReserveImport", ctx, adImport.OwnerID, adImport.Key, adImport.Fingerprint()).
			Return(domain.ImportRecord{RequestHash: otherContent.Fingerprint(), AdIDs: []domain.AdID{7}}, true, nil).Once()
		// No CreateAd expectation: the import is rejected as a whole.
		tx.On("Commit").Return(nil).Maybe()
		tx.On("Rollback").Return(nil).Maybe()

		adIDs, err := usecase.NewImportAdsFunc(dao)(ctx, importAdsRequest)

		require.ErrorIs(t, err, domain.ErrImportKeyReused)
		assert.Nil(t, adIDs)
	})

	t.Run("replays a completed import instead of creating the ads again", func(t *testing.T) {
		fingerprint := adImport.Fingerprint()
		dao := mockdao.New(t)
		tx := mockdao.New(t)
		dao.On("Begin", ctx).Return(tx, nil).Once()
		tx.On("ReserveImport", ctx, adImport.OwnerID, adImport.Key, fingerprint).
			Return(domain.ImportRecord{RequestHash: fingerprint, AdIDs: []domain.AdID{10, 11, 12}}, true, nil).Once()
		// No CreateAd expectation: a replay must not create any ad. The transaction wrote nothing, so committing or
		// rolling it back are both fine.
		tx.On("Commit").Return(nil).Maybe()
		tx.On("Rollback").Return(nil).Maybe()

		adIDs, err := usecase.NewImportAdsFunc(dao)(ctx, importAdsRequest)

		require.NoError(t, err)
		assert.Equal(t, []domain.AdID{10, 11, 12}, adIDs)
	})

	t.Run("stores the normalized title", func(t *testing.T) {
		dao := mockdao.New(t)
		tx := mockdao.New(t)
		dao.On("Begin", ctx).Return(tx, nil).Once()
		expectNewImport(ctx, tx, domain.AdImport{
			OwnerID: "user-1",
			Key:     importKey,
			Drafts:  []domain.AdDraft{{Title: "Mountain bike", PriceCents: 12000, PhotoURL: "https://example.com/photos/bike.jpg", OwnerID: "user-1"}},
		})
		tx.On("CreateAd", ctx, domain.AdDraft{Title: "Mountain bike", PriceCents: 12000, PhotoURL: "https://example.com/photos/bike.jpg", OwnerID: "user-1"}).Return(domain.AdID(10), nil).Once()
		tx.On("CompleteImport", ctx, domain.UserID("user-1"), importKey, []domain.AdID{10}).Return(nil).Once()
		tx.On("Commit").Return(nil).Once()
		tx.On("Rollback").Return(nil).Maybe()

		adIDs, err := usecase.NewImportAdsFunc(dao)(ctx, usecase.ImportAdsRequest{
			OwnerID:        "user-1",
			IdempotencyKey: importKey,
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
			IdempotencyKey: importKey,
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

	t.Run("rejects an empty import without opening a transaction", func(t *testing.T) {
		// nil is what the handler passes when the body has no ads field at all.
		for name, ads := range map[string][]usecase.ImportedAd{
			"no ads": nil,
			"empty":  {},
		} {
			t.Run(name, func(t *testing.T) {
				emptyImport := usecase.ImportAdsRequest{
					OwnerID:        "user-1",
					IdempotencyKey: importKey,
					Ads:            ads,
				}
				// No expectation: neither Begin nor CreateAd must be called.
				dao := mockdao.New(t)

				adIDs, err := usecase.NewImportAdsFunc(dao)(ctx, emptyImport)

				require.ErrorIs(t, err, domain.ErrInvalidAd)
				assert.Nil(t, adIDs)
			})
		}
	})
}
