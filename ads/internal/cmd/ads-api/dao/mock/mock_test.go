package mock_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain/ports"
)

var ctx = context.Background()

func TestMock_SearchAds(t *testing.T) {
	t.Run("returns configured ads", func(t *testing.T) {
		m := mock.New(t)
		req := ports.AdFilter{Title: "bike"}
		want := []domain.Ad{{ID: 1, Title: "bike", Price: 5000}}

		m.On("SearchAds", ctx, req).Return(want, nil)

		got, err := m.SearchAds(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("returns nil slice on error", func(t *testing.T) {
		m := mock.New(t)
		req := ports.AdFilter{Title: "bike"}
		boom := errors.New("db down")

		m.On("SearchAds", ctx, req).Return(nil, boom)

		got, err := m.SearchAds(ctx, req)

		assert.ErrorIs(t, err, boom)
		assert.Nil(t, got)
	})
}

func TestMock_GetAdByID(t *testing.T) {
	t.Run("returns configured ad", func(t *testing.T) {
		m := mock.New(t)
		want := domain.Ad{ID: 42, Title: "surfboard", Price: 15000}

		m.On("GetAdByID", ctx, domain.AdID(42)).Return(want, nil)

		got, err := m.GetAdByID(ctx, 42)

		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("returns ErrAdNotFound", func(t *testing.T) {
		m := mock.New(t)

		m.On("GetAdByID", ctx, domain.AdID(99)).Return(domain.Ad{}, domain.ErrAdNotFound)

		_, err := m.GetAdByID(ctx, 99)

		assert.ErrorIs(t, err, domain.ErrAdNotFound)
	})
}

func TestMock_CreateAd(t *testing.T) {
	t.Run("returns generated ID", func(t *testing.T) {
		m := mock.New(t)
		req := domain.AdDraft{
			Title:      "bike",
			PriceCents: 5000,
			PhotoURL:   "https://example.com/bike.jpg",
			OwnerID:    "user-1",
		}

		m.On("CreateAd", ctx, req).Return(domain.AdID(7), nil)

		got, err := m.CreateAd(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(7), got)
	})

	t.Run("returns ErrUnknownOwner when owner does not exist", func(t *testing.T) {
		m := mock.New(t)
		req := domain.AdDraft{
			Title:      "bike",
			PriceCents: 5000,
			PhotoURL:   "https://example.com/bike.jpg",
			OwnerID:    "ghost",
		}

		m.On("CreateAd", ctx, req).Return(domain.AdID(0), domain.ErrUnknownOwner)

		_, err := m.CreateAd(ctx, req)

		assert.ErrorIs(t, err, domain.ErrUnknownOwner)
	})
}

func TestMock_Begin(t *testing.T) {
	t.Run("returns configured transaction", func(t *testing.T) {
		m := mock.New(t)
		configured := mock.New(t)

		m.On("Begin", ctx).Return(configured, nil)

		tx, err := m.Begin(ctx)

		require.NoError(t, err)
		assert.Same(t, configured, tx)
	})

	t.Run("returns configured error", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("db down")

		m.On("Begin", ctx).Return(nil, boom)

		tx, err := m.Begin(ctx)

		assert.ErrorIs(t, err, boom)
		assert.Nil(t, tx)
	})
}

func TestMock_ReserveImport(t *testing.T) {
	hash := []byte("fingerprint")

	t.Run("returns configured stored import", func(t *testing.T) {
		m := mock.New(t)
		stored := domain.ImportRecord{RequestHash: hash, AdIDs: []domain.AdID{10, 11}}

		m.On("ReserveImport", ctx, domain.UserID("user-1"), "key-1", hash).Return(stored, true, nil)

		previous, found, err := m.ReserveImport(ctx, "user-1", "key-1", hash)

		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, stored, previous)
	})

	t.Run("returns configured error", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("db down")

		m.On("ReserveImport", ctx, domain.UserID("user-1"), "key-1", hash).Return(nil, false, boom)

		previous, found, err := m.ReserveImport(ctx, "user-1", "key-1", hash)

		assert.ErrorIs(t, err, boom)
		assert.False(t, found)
		assert.Zero(t, previous)
	})
}

func TestMock_CompleteImport(t *testing.T) {
	adIDs := []domain.AdID{10, 11}

	t.Run("returns nil on success", func(t *testing.T) {
		m := mock.New(t)

		m.On("CompleteImport", ctx, domain.UserID("user-1"), "key-1", adIDs).Return(nil)

		require.NoError(t, m.CompleteImport(ctx, "user-1", "key-1", adIDs))
	})

	t.Run("returns configured error", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("db down")

		m.On("CompleteImport", ctx, domain.UserID("user-1"), "key-1", adIDs).Return(boom)

		assert.ErrorIs(t, m.CompleteImport(ctx, "user-1", "key-1", adIDs), boom)
	})
}

func TestMock_LockPublisher(t *testing.T) {
	t.Run("returns nil on success", func(t *testing.T) {
		m := mock.New(t)

		m.On("LockPublisher", ctx, domain.UserID("user-1")).Return(nil)

		require.NoError(t, m.LockPublisher(ctx, "user-1"))
	})

	t.Run("returns configured error", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("db down")

		m.On("LockPublisher", ctx, domain.UserID("user-1")).Return(boom)

		assert.ErrorIs(t, m.LockPublisher(ctx, "user-1"), boom)
	})
}

func TestMock_LoadPublisher(t *testing.T) {
	t.Run("returns configured publisher", func(t *testing.T) {
		m := mock.New(t)
		want := domain.Publisher{ID: "user-1", RecentPublications: 3}

		m.On("LoadPublisher", ctx, domain.UserID("user-1"), time.Hour).Return(want, nil)

		got, err := m.LoadPublisher(ctx, "user-1", time.Hour)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("returns configured error", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("db down")

		m.On("LoadPublisher", ctx, domain.UserID("user-1"), time.Hour).Return(domain.Publisher{}, boom)

		_, err := m.LoadPublisher(ctx, "user-1", time.Hour)
		assert.ErrorIs(t, err, boom)
	})
}

func TestMock_RecordPublication(t *testing.T) {
	t.Run("returns nil on success", func(t *testing.T) {
		m := mock.New(t)

		m.On("RecordPublication", ctx, domain.UserID("user-1")).Return(nil)

		require.NoError(t, m.RecordPublication(ctx, "user-1"))
	})

	t.Run("returns configured error", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("db down")

		m.On("RecordPublication", ctx, domain.UserID("user-1")).Return(boom)

		assert.ErrorIs(t, m.RecordPublication(ctx, "user-1"), boom)
	})
}

func TestMock_Commit(t *testing.T) {
	t.Run("returns nil on success", func(t *testing.T) {
		m := mock.New(t)

		m.On("Commit").Return(nil)

		require.NoError(t, m.Commit())
	})

	t.Run("returns configured error", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("commit failed")

		m.On("Commit").Return(boom)

		assert.ErrorIs(t, m.Commit(), boom)
	})
}

func TestMock_Rollback(t *testing.T) {
	t.Run("returns nil on success", func(t *testing.T) {
		m := mock.New(t)

		m.On("Rollback").Return(nil)

		require.NoError(t, m.Rollback())
	})

	t.Run("returns configured error", func(t *testing.T) {
		m := mock.New(t)
		boom := errors.New("rollback failed")

		m.On("Rollback").Return(boom)

		assert.ErrorIs(t, m.Rollback(), boom)
	})
}
