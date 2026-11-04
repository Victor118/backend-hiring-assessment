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

// expectPublisherLoaded returns a repository whose transaction expects the owner to be locked, then loaded with the
// given number of recent publications. The test adds what follows the loading.
func expectPublisherLoaded(t *testing.T, ctx context.Context, ownerID domain.UserID, recentPublications int) (repo, tx *mockdao.Mock) {
	t.Helper()

	repo = mockdao.New(t)
	tx = mockdao.New(t)
	repo.On("Begin", ctx).Return(tx, nil).Once()
	lockPublisher := tx.On("LockPublisher", ctx, ownerID).Return(nil).Once()
	tx.On("LoadPublisher", ctx, ownerID, domain.PublicationWindow).
		Return(domain.Publisher{ID: ownerID, RecentPublications: recentPublications}, nil).Once().NotBefore(lockPublisher)

	return repo, tx
}

// expectPublicationCommitted expects the publication of the owner to be recorded, then the transaction to commit.
func expectPublicationCommitted(ctx context.Context, tx *mockdao.Mock, ownerID domain.UserID) {
	recordPublication := tx.On("RecordPublication", ctx, ownerID).Return(nil).Once()
	tx.On("Commit").Return(nil).Once().NotBefore(recordPublication)
	// The deferred Rollback runs even after a successful Commit, where it does nothing.
	tx.On("Rollback").Return(nil).Maybe()
}

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
		repo := mockdao.New(t)
		tx := mockdao.New(t)
		repo.On("Begin", ctx).Return(tx, nil).Once()
		// The publisher is loaded under its lock: otherwise two concurrent creations would both see room left.
		lockPublisher := tx.On("LockPublisher", ctx, ucReq.OwnerID).Return(nil).Once()
		publisher := domain.Publisher{ID: ucReq.OwnerID, RecentPublications: 0}
		tx.On("LoadPublisher", ctx, ucReq.OwnerID, domain.PublicationWindow).
			Return(publisher, nil).Once().NotBefore(lockPublisher)
		tx.On("CreateAd", ctx, draft).Return(domain.AdID(7), nil).Once()
		recordPublication := tx.On("RecordPublication", ctx, ucReq.OwnerID).Return(nil).Once()
		tx.On("Commit").Return(nil).Once().NotBefore(recordPublication)
		// The deferred Rollback runs even after a successful Commit, where it does nothing.
		tx.On("Rollback").Return(nil).Maybe()

		got, err := usecase.NewCreateAdFunc(repo)(ctx, ucReq)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(7), got)
	})

	t.Run("rolls back and returns the error when the ad cannot be created", func(t *testing.T) {
		repo, tx := expectPublisherLoaded(t, ctx, ucReq.OwnerID, 0)
		dbErr := errors.New("connection reset")
		tx.On("CreateAd", ctx, draft).Return(domain.AdID(0), dbErr).Once()
		// No RecordPublication nor Commit: the publication must not be recorded for an ad that was not created.
		tx.On("Rollback").Return(nil).Once()

		_, err := usecase.NewCreateAdFunc(repo)(ctx, ucReq)

		require.ErrorIs(t, err, dbErr)
	})

	t.Run("accepts a free ad", func(t *testing.T) {
		free := ucReq
		free.PriceCents = 0
		freeDraft := draft
		freeDraft.PriceCents = 0

		repo, tx := expectPublisherLoaded(t, ctx, ucReq.OwnerID, 0)
		tx.On("CreateAd", ctx, freeDraft).Return(domain.AdID(8), nil).Once()
		expectPublicationCommitted(ctx, tx, ucReq.OwnerID)

		got, err := usecase.NewCreateAdFunc(repo)(ctx, free)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(8), got)
	})

	t.Run("stores the normalized title", func(t *testing.T) {
		padded := ucReq
		padded.Title = "  mountain bike  "

		repo, tx := expectPublisherLoaded(t, ctx, ucReq.OwnerID, 0)
		tx.On("CreateAd", ctx, draft).Return(domain.AdID(7), nil).Once()
		expectPublicationCommitted(ctx, tx, ucReq.OwnerID)

		got, err := usecase.NewCreateAdFunc(repo)(ctx, padded)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(7), got)
	})

	t.Run("rejects the ad when the publisher has reached its quota", func(t *testing.T) {
		repo, tx := expectPublisherLoaded(t, ctx, ucReq.OwnerID, domain.PublicationQuota)
		// No CreateAd, RecordPublication nor Commit: the ad must not be published.
		tx.On("Rollback").Return(nil).Once()

		adID, err := usecase.NewCreateAdFunc(repo)(ctx, ucReq)

		require.ErrorIs(t, err, domain.ErrQuotaExceeded)
		assert.Zero(t, adID)
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
			m := mockdao.New(t)

			_, err := usecase.NewCreateAdFunc(m)(ctx, req)

			require.ErrorIs(t, err, domain.ErrInvalidAd)
		})
	}

	// Each failing step must stop the creation: the mocks being strict, any call after the failing step fails the test.
	failingSteps := []struct {
		name  string
		setup func(repo, tx *mockdao.Mock, boom error)
	}{
		{
			name: "begin",
			setup: func(repo, _ *mockdao.Mock, boom error) {
				// No transaction was opened, so there is nothing to roll back.
				repo.On("Begin", ctx).Return(nil, boom).Once()
			},
		},
		{
			name: "lock publisher",
			setup: func(repo, tx *mockdao.Mock, boom error) {
				repo.On("Begin", ctx).Return(tx, nil).Once()
				tx.On("LockPublisher", ctx, ucReq.OwnerID).Return(boom).Once()
				tx.On("Rollback").Return(nil).Once()
			},
		},
		{
			name: "load publisher",
			setup: func(repo, tx *mockdao.Mock, boom error) {
				repo.On("Begin", ctx).Return(tx, nil).Once()
				tx.On("LockPublisher", ctx, ucReq.OwnerID).Return(nil).Once()
				tx.On("LoadPublisher", ctx, ucReq.OwnerID, domain.PublicationWindow).Return(domain.Publisher{}, boom).Once()
				tx.On("Rollback").Return(nil).Once()
			},
		},
		{
			name: "record publication",
			setup: func(repo, tx *mockdao.Mock, boom error) {
				repo.On("Begin", ctx).Return(tx, nil).Once()
				tx.On("LockPublisher", ctx, ucReq.OwnerID).Return(nil).Once()
				tx.On("LoadPublisher", ctx, ucReq.OwnerID, domain.PublicationWindow).Return(domain.Publisher{ID: ucReq.OwnerID}, nil).Once()
				tx.On("CreateAd", ctx, draft).Return(domain.AdID(7), nil).Once()
				tx.On("RecordPublication", ctx, ucReq.OwnerID).Return(boom).Once()
				tx.On("Rollback").Return(nil).Once()
			},
		},
		{
			name: "commit",
			setup: func(repo, tx *mockdao.Mock, boom error) {
				repo.On("Begin", ctx).Return(tx, nil).Once()
				tx.On("LockPublisher", ctx, ucReq.OwnerID).Return(nil).Once()
				tx.On("LoadPublisher", ctx, ucReq.OwnerID, domain.PublicationWindow).Return(domain.Publisher{ID: ucReq.OwnerID}, nil).Once()
				tx.On("CreateAd", ctx, draft).Return(domain.AdID(7), nil).Once()
				tx.On("RecordPublication", ctx, ucReq.OwnerID).Return(nil).Once()
				tx.On("Commit").Return(boom).Once()
				// The deferred Rollback still runs after a failed Commit.
				tx.On("Rollback").Return(nil).Maybe()
			},
		},
	}
	for _, step := range failingSteps {
		t.Run("returns the error when the step fails: "+step.name, func(t *testing.T) {
			repo := mockdao.New(t)
			tx := mockdao.New(t)
			boom := errors.New(step.name + " failed")
			step.setup(repo, tx, boom)

			adID, err := usecase.NewCreateAdFunc(repo)(ctx, ucReq)

			require.ErrorIs(t, err, boom)
			// No ID is returned for an ad that may not exist.
			assert.Zero(t, adID)
		})
	}
}
